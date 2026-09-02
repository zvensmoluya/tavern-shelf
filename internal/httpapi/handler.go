package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openai/tavern-shelf/internal/app"
	"github.com/openai/tavern-shelf/internal/library"
	"github.com/openai/tavern-shelf/internal/store"
	"github.com/openai/tavern-shelf/internal/transfer"
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
	registerConnectorManagement(mux, application)
	connectorHandler := ConnectorHandler(application)
	mux.Handle("GET /connector/", connectorHandler)
	mux.Handle("POST /connector/", connectorHandler)
	mux.Handle("OPTIONS /connector/", connectorHandler)
	var desktop DesktopActions
	if len(desktopActions) > 0 {
		desktop = desktopActions[0]
	}
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		desktopStatus := map[string]any{"available": desktop != nil, "autoStart": false}
		inboxes := application.Inboxes()
		inboxDetails := make([]map[string]string, 0, len(inboxes))
		for _, directory := range inboxes {
			inboxDetails = append(inboxDetails, map[string]string{"path": directory, "mode": application.InboxMode(directory)})
		}
		if desktop != nil {
			enabled, err := desktop.AutoStartEnabled()
			if err == nil {
				desktopStatus["autoStart"] = enabled
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"scanner":     application.Scanner.Status(),
			"oneShotScan": application.OneShotScanStatus(),
			"paths": map[string]any{
				"inbox": application.Paths.Inbox, "inboxes": inboxes, "inboxDetails": inboxDetails,
				"library": application.Paths.Library, "appData": application.Paths.AppData, "trash": application.Paths.Trash,
			},
			"desktop": desktopStatus,
		})
	})
	mux.HandleFunc("POST /api/scans", func(w http.ResponseWriter, r *http.Request) {
		path, ok := decodeDirectory(w, r)
		if !ok {
			return
		}
		status, err := application.StartScanOnce(r.Context(), path)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusAccepted, status)
	})
	mux.HandleFunc("POST /api/imports", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, app.MaxUploadSize+(1<<20))
		file, header, err := r.FormFile("file")
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				writeError(w, http.StatusRequestEntityTooLarge, app.ErrUploadTooLarge)
				return
			}
			writeError(w, http.StatusBadRequest, errors.New("a PNG or JSON file is required"))
			return
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		defer file.Close()
		result, err := application.ImportUpload(r.Context(), header.Filename, io.LimitReader(file, app.MaxUploadSize+1))
		if err != nil {
			if errors.Is(err, app.ErrUploadTooLarge) {
				writeError(w, http.StatusRequestEntityTooLarge, err)
				return
			}
			writeError(w, http.StatusBadRequest, err)
			return
		}
		id := result.Character.ID
		if result.Resource.ID != "" {
			id = result.Resource.ID
		}
		statusCode := http.StatusCreated
		if result.Duplicate {
			statusCode = http.StatusOK
		}
		writeJSON(w, statusCode, map[string]any{
			"id": id, "kind": result.Kind, "name": result.Name, "duplicate": result.Duplicate,
		})
	})
	mux.HandleFunc("GET /api/backup", func(w http.ResponseWriter, r *http.Request) {
		file, err := os.CreateTemp(application.Paths.Staging, "backup-*.zip")
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("create backup staging file: %w", err))
			return
		}
		path := file.Name()
		defer os.Remove(path)
		if _, err := application.WriteBackup(r.Context(), file); err != nil {
			_ = file.Close()
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := file.Close(); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("close backup archive: %w", err))
			return
		}
		file, err = os.Open(path)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("open backup archive: %w", err))
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("inspect backup archive: %w", err))
			return
		}
		name := "Tavern-Shelf-Backup-" + time.Now().Format("20060102-150405") + ".zip"
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
		http.ServeContent(w, r, name, info.ModTime(), file)
	})
	mux.HandleFunc("POST /api/backups/restore", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, app.MaxBackupSize+(1<<20))
		file, _, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, errors.New("a Tavern Shelf backup ZIP is required"))
			return
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		defer file.Close()
		summary, err := application.RestoreBackup(r.Context(), file)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, summary)
	})
	mux.HandleFunc("GET /api/trash", func(w http.ResponseWriter, r *http.Request) {
		items, err := application.ListTrash()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	})
	mux.HandleFunc("POST /api/trash/{id}/restore", func(w http.ResponseWriter, r *http.Request) {
		summary, err := application.RestoreTrash(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, summary)
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
	mux.HandleFunc("PUT /api/characters/{id}/organization", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Favorite      bool     `json:"favorite"`
			Note          string   `json:"note"`
			CollectionIDs []string `json:"collectionIds"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("invalid character organization"))
			return
		}
		if err := application.OrganizeCharacter(r.Context(), r.PathValue("id"), library.CharacterOrganization{
			Favorite: request.Favorite, Note: request.Note, CollectionIDs: request.CollectionIDs,
		}); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeError(w, http.StatusBadRequest, err)
			return
		}
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
	mux.HandleFunc("GET /api/resources", func(w http.ResponseWriter, r *http.Request) {
		kind := strings.TrimSpace(r.URL.Query().Get("kind"))
		if kind != "" && kind != "worldbook" && kind != "preset" {
			writeError(w, http.StatusBadRequest, errors.New("invalid resource kind"))
			return
		}
		resources, err := application.ListResources(r.Context(), kind)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, resources)
	})
	mux.HandleFunc("GET /api/collections", func(w http.ResponseWriter, r *http.Request) {
		collections, err := application.Collections(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, collections)
	})
	mux.HandleFunc("POST /api/collections", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("valid collection name is required"))
			return
		}
		collection, err := application.CreateCollection(r.Context(), request.Name)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, collection)
	})
	mux.HandleFunc("PUT /api/collections/{id}", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("valid collection name is required"))
			return
		}
		if err := application.RenameCollection(r.Context(), r.PathValue("id"), request.Name); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeError(w, http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /api/collections/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := application.DeleteCollection(r.Context(), r.PathValue("id")); err != nil {
			writeStoreError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/resources/{id}", func(w http.ResponseWriter, r *http.Request) {
		resource, err := application.GetResource(r.Context(), r.PathValue("id"))
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resource)
	})
	mux.HandleFunc("DELETE /api/resources/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := application.DeleteResource(r.Context(), r.PathValue("id")); err != nil {
			writeStoreError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/resources/{id}/source", func(w http.ResponseWriter, r *http.Request) {
		resource, path, err := application.ResourceSourcePath(r.Context(), r.PathValue("id"))
		if err != nil {
			writeStoreError(w, err)
			return
		}
		file, err := os.Open(path)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("open managed resource source: %w", err))
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("inspect managed resource source: %w", err))
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.URL.Query().Has("download") {
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeDownloadName(resource.SourceFilename)))
		}
		http.ServeContent(w, r, resource.SourceFilename, info.ModTime(), file)
	})
	mux.HandleFunc("POST /api/transfers", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Kind string `json:"kind"`
			ID   string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || strings.TrimSpace(request.ID) == "" {
			writeError(w, http.StatusBadRequest, errors.New("valid transfer resource kind and ID are required"))
			return
		}
		session, err := application.Transfers.Create(r.Context(), request.Kind, request.ID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, session)
	})
	mux.HandleFunc("DELETE /api/transfers/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := application.Transfers.Revoke(r.PathValue("id")); err != nil {
			if errors.Is(err, transfer.ErrNotFound) {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
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
