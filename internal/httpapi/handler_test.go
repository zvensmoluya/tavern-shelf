package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/openai/tavern-shelf/internal/app"
)

func TestInboxDirectoryAPI(t *testing.T) {
	shelf, err := app.Open(app.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	handler, err := Handler(shelf)
	if err != nil {
		t.Fatal(err)
	}
	external := t.TempDir()

	requestJSON(t, handler, http.MethodPost, "/api/inboxes", map[string]string{"path": external}, http.StatusNoContent)

	request := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, body = %s", response.Code, response.Body.String())
	}
	var status struct {
		Paths struct {
			Inboxes []string `json:"inboxes"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if len(status.Paths.Inboxes) != 2 || status.Paths.Inboxes[1] != external {
		t.Fatalf("unexpected Inbox status: %#v", status.Paths.Inboxes)
	}

	requestJSON(t, handler, http.MethodDelete, "/api/inboxes", map[string]string{"path": shelf.Paths.Inbox}, http.StatusNoContent)
	requestJSON(t, handler, http.MethodDelete, "/api/inboxes", map[string]string{"path": external}, http.StatusBadRequest)
	if directories := shelf.Inboxes(); len(directories) != 1 || directories[0] != external {
		t.Fatalf("unexpected final Inbox settings: %#v", directories)
	}
	if err := shelf.Scanner.ScanOnce(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
}

func requestJSON(t *testing.T, handler http.Handler, method, target string, body any, expected int) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, target, bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != expected {
		t.Fatalf("%s %s code = %d, want %d, body = %s", method, target, response.Code, expected, response.Body.String())
	}
}
