package transfer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestTransferManifestAndSource(t *testing.T) {
	path := t.TempDir() + "/card.json"
	raw := []byte(`{"name":"Lantern"}`)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	server := NewServer(func(context.Context, string, string) (Source, error) {
		return Source{
			Kind: "character", ID: "card-1", Name: "Lantern", Filename: "lantern.json",
			Path: path, Size: int64(len(raw)), SHA256: strings.Repeat("a", 64),
		}, nil
	})
	t.Cleanup(func() { _ = server.Close() })

	session, err := server.Create(context.Background(), "character", "card-1")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/v1/transfers/"+session.ID, nil)
	request.Host = "192.168.1.8:4567"
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("manifest code = %d, body = %s", response.Code, response.Body.String())
	}
	var manifest Manifest
	if err := json.Unmarshal(response.Body.Bytes(), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Protocol != Protocol || manifest.Version != Version || manifest.Kind != "character" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	if manifest.SourceURL != "http://192.168.1.8:4567/v1/transfers/"+session.ID+"/source" {
		t.Fatalf("unexpected source URL: %q", manifest.SourceURL)
	}

	request = httptest.NewRequest(http.MethodGet, "/v1/transfers/"+session.ID+"/source", nil)
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != string(raw) {
		t.Fatalf("source code = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Disposition") != `attachment; filename="lantern.json"` {
		t.Fatalf("unexpected content disposition: %q", response.Header().Get("Content-Disposition"))
	}
}

func TestExpiredAndRevokedTransfersAreUnavailable(t *testing.T) {
	path := t.TempDir() + "/preset.json"
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	server := NewServer(func(context.Context, string, string) (Source, error) {
		return Source{Kind: "preset", ID: "preset-1", Name: "Preset", Filename: "preset.json", Path: path, Size: 2, SHA256: strings.Repeat("b", 64)}, nil
	})
	server.now = func() time.Time { return now }
	t.Cleanup(func() { _ = server.Close() })

	revoked, err := server.Create(context.Background(), "preset", "preset-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Revoke(revoked.ID); err != nil {
		t.Fatal(err)
	}
	assertTransferMissing(t, server, revoked.ID)
	if err := server.Revoke(revoked.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second revoke error = %v", err)
	}

	expired, err := server.Create(context.Background(), "preset", "preset-1")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(11 * time.Minute)
	assertTransferMissing(t, server, expired.ID)
}

func assertTransferMissing(t *testing.T, server *Server, id string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/v1/transfers/"+id, nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("transfer %q code = %d, want 404", id, response.Code)
	}
}
