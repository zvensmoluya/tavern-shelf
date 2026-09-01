package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openai/tavern-shelf/internal/app"
	"github.com/openai/tavern-shelf/internal/importer"
	"github.com/openai/tavern-shelf/internal/library"
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

func TestResourceAPI(t *testing.T) {
	shelf, err := app.Open(app.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	raw := []byte(`{"entries":{"0":{"key":["city"],"comment":"City","content":"A city."}}}`)
	source := shelf.Paths.Inbox + string(filepath.Separator) + "City.json"
	if err := os.WriteFile(source, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := importer.New(shelf.Paths, shelf.Store).Import(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	handler, err := Handler(shelf)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/resources?kind=worldbook", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("resource list code = %d, body = %s", response.Code, response.Body.String())
	}
	var resources []library.Resource
	if err := json.Unmarshal(response.Body.Bytes(), &resources); err != nil {
		t.Fatal(err)
	}
	if len(resources) != 1 || resources[0].Kind != library.ResourceWorldbook || resources[0].SourceURL == "" {
		t.Fatalf("unexpected resource response: %#v", resources)
	}
	request = httptest.NewRequest(http.MethodGet, resources[0].SourceURL+"?download=1", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), raw) {
		t.Fatalf("resource source response code = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestCreateAndRevokeTransferAPI(t *testing.T) {
	shelf, err := app.Open(app.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	raw := []byte(`{"spec":"chara_card_v2","spec_version":"2.0","data":{"name":"Share Me"}}`)
	source := filepath.Join(shelf.Paths.Inbox, "share-me.json")
	if err := os.WriteFile(source, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := importer.New(shelf.Paths, shelf.Store).Import(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := Handler(shelf)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]string{"kind": "character", "id": result.Character.ID})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/transfers", bytes.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create transfer code = %d, body = %s", response.Code, response.Body.String())
	}
	var session struct {
		ID        string    `json:"id"`
		URL       string    `json:"url"`
		Kind      string    `json:"kind"`
		ExpiresAt time.Time `json:"expiresAt"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	if session.ID == "" || session.URL == "" || session.Kind != "character" || !session.ExpiresAt.After(time.Now()) {
		t.Fatalf("unexpected transfer session: %#v", session)
	}

	request = httptest.NewRequest(http.MethodDelete, "/api/transfers/"+session.ID, nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("revoke transfer code = %d, body = %s", response.Code, response.Body.String())
	}
}
