package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/zvensmoluya/tavern-shelf/internal/app"
	"github.com/zvensmoluya/tavern-shelf/internal/connector"
	"github.com/zvensmoluya/tavern-shelf/internal/store"
)

func registerConnectorManagement(mux *http.ServeMux, application *app.App) {
	mux.HandleFunc("GET /api/connector", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, application.Connector.Status())
	})
	mux.HandleFunc("POST /api/connector/pairing", func(w http.ResponseWriter, _ *http.Request) {
		pairing, err := application.Connector.BeginPairing()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, pairing)
	})
	mux.HandleFunc("DELETE /api/connector/pairing", func(w http.ResponseWriter, _ *http.Request) {
		if err := application.Connector.Revoke(); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// ConnectorHandler exposes the deliberately small API used by local client adapters.
func ConnectorHandler(application *app.App) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /connector/v1/status", func(w http.ResponseWriter, _ *http.Request) {
		status := application.Connector.Status()
		writeJSON(w, http.StatusOK, map[string]any{
			"protocol": status.Protocol, "version": status.Version, "paired": status.Paired,
		})
	})
	mux.HandleFunc("POST /connector/v1/pair", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Code          string `json:"code"`
			ClientName    string `json:"clientName"`
			ClientVersion string `json:"clientVersion"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("invalid pairing request"))
			return
		}
		token, err := application.Connector.Pair(strings.TrimSpace(request.Code), strings.TrimSpace(request.ClientName), strings.TrimSpace(request.ClientVersion))
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, connector.ErrInvalidCode) {
				status = http.StatusUnauthorized
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"token": token})
	})
	mux.Handle("GET /connector/v1/characters", requireConnectorAuth(application, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		characters, err := application.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		result := make([]map[string]any, 0, len(characters))
		for _, character := range characters {
			sourceURL := "/connector/v1/characters/" + character.ID + "/source"
			result = append(result, map[string]any{
				"id": character.ID, "name": character.Name, "creator": character.Creator,
				"tags": character.Tags, "sourceFormat": character.SourceFormat,
				"sourceIsImage": character.SourceIsImage, "sourceFilename": character.SourceFilename,
				"sourceSize": character.SourceSize, "importedAt": character.ImportedAt,
				"sourceUrl": sourceURL,
			})
		}
		writeJSON(w, http.StatusOK, result)
	})))
	mux.Handle("GET /connector/v1/characters/{id}/source", requireConnectorAuth(application, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		character, sourcePath, err := application.SourcePath(r.Context(), r.PathValue("id"))
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, err)
			} else {
				writeError(w, http.StatusInternalServerError, err)
			}
			return
		}
		file, err := os.Open(sourcePath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("open managed source: %w", err))
			return
		}
		defer file.Close()
		contentType := mime.TypeByExtension(filepath.Ext(character.SourceFilename))
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeDownloadName(character.SourceFilename)))
		_, _ = io.Copy(w, file)
	})))
	mux.Handle("POST /connector/v1/imports", requireConnectorAuth(application, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, errors.New("a PNG or JSON character card is required"))
			return
		}
		defer file.Close()
		result, err := application.ImportCharacterUpload(r.Context(), header.Filename, file)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, app.ErrUploadTooLarge) {
				status = http.StatusRequestEntityTooLarge
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"id": result.Character.ID, "kind": "character", "name": result.Character.Name, "duplicate": result.Duplicate,
		})
	})))
	return connectorCORS(mux)
}

func requireConnectorAuth(application *app.App, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if !application.Connector.Authorize(token) {
			writeError(w, http.StatusUnauthorized, connector.ErrUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func connectorCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		origin := r.Header.Get("Origin")
		if origin != "" {
			if !isLoopbackOrigin(origin) {
				writeError(w, http.StatusForbidden, errors.New("connector origin is not allowed"))
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopbackOrigin(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Path != "" {
		return false
	}
	host := parsed.Hostname()
	return strings.EqualFold(host, "localhost") || (net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback())
}
