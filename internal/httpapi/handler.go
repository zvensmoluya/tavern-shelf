package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openai/tavern-shelf/internal/app"
	"github.com/openai/tavern-shelf/internal/store"
	"github.com/openai/tavern-shelf/internal/webui"
)

type DesktopActions interface {
	OpenInbox(path string) error
	ChooseInbox() (string, error)
	AutoStartEnabled() (bool, error)
	SetAutoStart(enabled bool) error
}

func Handler(application *app.App, desktopActions ...DesktopActions) (http.Handler, error) {
	assets, err := fs.Sub(webui.Assets, "static")
	if err != nil {
		return nil, fmt.Errorf("open embedded UI: %w", err)
	}
	mux := http.NewServeMux()
	var desktop DesktopActions
	if len(desktopActions) > 0 {
		desktop = desktopActions[0]
	}
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		desktopStatus := map[string]any{"available": desktop != nil, "autoStart": false}
		if desktop != nil {
			enabled, err := desktop.AutoStartEnabled()
			if err == nil {
				desktopStatus["autoStart"] = enabled
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"scanner": application.Scanner.Status(),
			"paths": map[string]any{
				"inbox": application.Paths.Inbox, "inboxes": application.Inboxes(),
				"library": application.Paths.Library, "appData": application.Paths.AppData, "trash": application.Paths.Trash,
			},
			"desktop": desktopStatus,
		})
	})
	mux.HandleFunc("POST /api/inboxes", func(w http.ResponseWriter, r *http.Request) {
		path, ok := decodeDirectory(w, r)
		if !ok {
			return
		}
		if err := application.AddInbox(r.Context(), path); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /api/inboxes", func(w http.ResponseWriter, r *http.Request) {
		path, ok := decodeDirectory(w, r)
		if !ok {
			return
		}
		if err := application.RemoveInbox(r.Context(), path); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/desktop/choose-inbox", func(w http.ResponseWriter, r *http.Request) {
		if desktop == nil {
			writeError(w, http.StatusNotImplemented, errors.New("desktop integration is not available in headless mode"))
			return
		}
		path, err := desktop.ChooseInbox()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"path": path})
	})
	mux.HandleFunc("POST /api/desktop/open-inbox", func(w http.ResponseWriter, r *http.Request) {
		if desktop == nil {
			writeError(w, http.StatusNotImplemented, errors.New("desktop integration is not available in headless mode"))
			return
		}
		path, ok := decodeDirectory(w, r)
		if !ok {
			return
		}
		if !application.HasInbox(path) {
			writeError(w, http.StatusBadRequest, errors.New("Inbox directory is not configured"))
			return
		}
		if err := desktop.OpenInbox(path); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /api/desktop/autostart", func(w http.ResponseWriter, r *http.Request) {
		if desktop == nil {
			writeError(w, http.StatusNotImplemented, errors.New("desktop integration is not available in headless mode"))
			return
		}
		var request struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("invalid auto-start setting"))
			return
		}
		if err := desktop.SetAutoStart(request.Enabled); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/characters", func(w http.ResponseWriter, r *http.Request) {
		characters, err := application.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, characters)
	})
	mux.HandleFunc("GET /api/characters/{id}", func(w http.ResponseWriter, r *http.Request) {
		character, err := application.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, character)
	})
	mux.HandleFunc("DELETE /api/characters/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := application.Delete(r.Context(), r.PathValue("id")); err != nil {
			writeStoreError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/characters/{id}/source", func(w http.ResponseWriter, r *http.Request) {
		character, path, err := application.SourcePath(r.Context(), r.PathValue("id"))
		if err != nil {
			writeStoreError(w, err)
			return
		}
		file, err := os.Open(path)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("open managed source: %w", err))
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("inspect managed source: %w", err))
			return
		}
		contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		if r.URL.Query().Has("download") {
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeDownloadName(character.SourceFilename)))
		}
		http.ServeContent(w, r, character.SourceFilename, info.ModTime(), file)
	})
	fileServer := http.FileServer(http.FS(assets))
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", fileServer))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		index, err := fs.ReadFile(assets, "index.html")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
	return securityHeaders(mux), nil
}

func decodeDirectory(w http.ResponseWriter, r *http.Request) (string, bool) {
	var request struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || strings.TrimSpace(request.Path) == "" {
		writeError(w, http.StatusBadRequest, errors.New("valid Inbox directory path is required"))
		return "", false
	}
	return request.Path, true
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeError(w, http.StatusInternalServerError, err)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func safeDownloadName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune("\\/:*?\"<>|", r) {
			return '_'
		}
		return r
	}, name)
	if name == "" {
		return "character-card-" + time.Now().Format("20060102")
	}
	return name
}
