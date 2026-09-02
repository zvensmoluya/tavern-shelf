package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
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

func TestDragImportAPI(t *testing.T) {
	shelf, err := app.Open(app.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	handler, err := Handler(shelf)
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "dragged.json")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte(`{"spec":"chara_card_v2","data":{"name":"Dragged","description":"Dropped safely"}}`))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/imports", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("import code = %d, body = %s", response.Code, response.Body.String())
	}
	var result struct {
		ID        string `json:"id"`
		Kind      string `json:"kind"`
		Duplicate bool   `json:"duplicate"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.ID == "" || result.Kind != "character" || result.Duplicate {
		t.Fatalf("unexpected import response: %#v", result)
	}
}

func TestOneShotScanAPILeavesSourceInPlace(t *testing.T) {
	external := t.TempDir()
	source := filepath.Join(external, "one-shot.json")
	if err := os.WriteFile(source, []byte(`{"spec":"chara_card_v2","data":{"name":"One Shot","description":"Keep me"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	shelf, err := app.Open(app.Options{DataDir: t.TempDir(), StableFor: 5 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	handler, err := Handler(shelf)
	if err != nil {
		t.Fatal(err)
	}
	requestJSON(t, handler, http.MethodPost, "/api/scans", map[string]string{"path": external}, http.StatusAccepted)
	deadline := time.Now().Add(2 * time.Second)
	for shelf.OneShotScanStatus().Running && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if finished := shelf.OneShotScanStatus(); finished.Imported != 1 || finished.Failed != 0 {
		t.Fatalf("unexpected scan result: %#v", finished)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("one-time scan removed source: %v", err)
	}
}

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
			Inboxes      []string `json:"inboxes"`
			InboxDetails []struct {
				Path string `json:"path"`
				Mode string `json:"mode"`
			} `json:"inboxDetails"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if len(status.Paths.Inboxes) != 2 || status.Paths.Inboxes[1] != external {
		t.Fatalf("unexpected Inbox status: %#v", status.Paths.Inboxes)
	}
	if len(status.Paths.InboxDetails) != 2 || status.Paths.InboxDetails[0].Mode != "move" || status.Paths.InboxDetails[1].Mode != "copy" {
		t.Fatalf("unexpected Inbox modes: %#v", status.Paths.InboxDetails)
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

func TestBackupRestoreAndTrashAPI(t *testing.T) {
	shelf, err := app.Open(app.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	result, err := shelf.ImportUpload(context.Background(), "backup-me.json", bytes.NewBufferString(`{"spec":"chara_card_v2","data":{"name":"Backup Me"}}`))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := Handler(shelf)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/backup", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/zip" || response.Body.Len() == 0 {
		t.Fatalf("backup response code=%d type=%q size=%d", response.Code, response.Header().Get("Content-Type"), response.Body.Len())
	}
	backup := append([]byte(nil), response.Body.Bytes()...)

	request = httptest.NewRequest(http.MethodDelete, "/api/characters/"+result.Character.ID, nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete before restore code=%d body=%s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/api/trash", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var trash []app.TrashItem
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &trash) != nil || len(trash) != 1 {
		t.Fatalf("Trash response code=%d body=%s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/api/trash/"+trash[0].ID+"/restore", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("Trash restore code=%d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodDelete, "/api/characters/"+result.Character.ID, nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("second delete code=%d body=%s", response.Code, response.Body.String())
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "Shelf.zip")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(backup)
	_ = writer.Close()
	request = httptest.NewRequest(http.MethodPost, "/api/backups/restore", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("backup restore code=%d body=%s", response.Code, response.Body.String())
	}
	var summary app.RestoreSummary
	if err := json.Unmarshal(response.Body.Bytes(), &summary); err != nil || summary.Imported != 1 {
		t.Fatalf("unexpected restore summary: %#v, %v", summary, err)
	}
}

func TestConnectorPairAuthorizeCORSAndImport(t *testing.T) {
	shelf, err := app.Open(app.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	pairing, err := shelf.Connector.BeginPairing()
	if err != nil {
		t.Fatal(err)
	}
	handler := ConnectorHandler(shelf)
	raw, _ := json.Marshal(map[string]string{"code": pairing.Code, "clientName": "ST Test", "clientVersion": "1.18.0"})
	request := httptest.NewRequest(http.MethodPost, "/connector/v1/pair", bytes.NewReader(raw))
	request.Header.Set("Origin", "http://127.0.0.1:8000")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var paired struct {
		Token string `json:"token"`
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &paired) != nil || paired.Token == "" {
		t.Fatalf("pair response code=%d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/connector/v1/characters", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized list code=%d", response.Code)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "from-st.json")
	_, _ = part.Write([]byte(`{"spec":"chara_card_v2","data":{"name":"From ST","description":"Safe"}}`))
	_ = writer.Close()
	request = httptest.NewRequest(http.MethodPost, "/connector/v1/imports", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+paired.Token)
	request.Header.Set("Origin", "http://localhost:8000")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:8000" {
		t.Fatalf("import response code=%d body=%s", response.Code, response.Body.String())
	}

	body.Reset()
	writer = multipart.NewWriter(&body)
	part, _ = writer.CreateFormFile("file", "world.json")
	_, _ = part.Write([]byte(`{"entries":{"0":{"key":["city"],"content":"City"}}}`))
	_ = writer.Close()
	request = httptest.NewRequest(http.MethodPost, "/connector/v1/imports", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+paired.Token)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	resources, listErr := shelf.ListResources(context.Background(), "")
	if response.Code != http.StatusBadRequest || listErr != nil || len(resources) != 0 {
		t.Fatalf("standalone resource import code=%d resources=%d err=%v", response.Code, len(resources), listErr)
	}

	request = httptest.NewRequest(http.MethodGet, "/connector/v1/characters", nil)
	request.Header.Set("Authorization", "Bearer "+paired.Token)
	request.Header.Set("Origin", "https://evil.example")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("forbidden origin code=%d", response.Code)
	}
}
