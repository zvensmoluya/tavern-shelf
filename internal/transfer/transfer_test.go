package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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
			Path: path, Size: int64(len(raw)), SHA256: sourceHash(raw),
		}, nil
	})
	t.Cleanup(func() { _ = server.Close() })

	session, err := server.Create(context.Background(), "character", "card-1")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/v1/transfers/"+session.ID, nil)
	sessionURL, err := url.Parse(session.URL)
	if err != nil {
		t.Fatal(err)
	}
	request.Host = sessionURL.Host
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
	if manifest.SourceURL != session.URL+"/source" {
		t.Fatalf("unexpected source URL: %q", manifest.SourceURL)
	}
	request = httptest.NewRequest(http.MethodGet, "/v1/transfers/"+session.ID, nil)
	request.Host = "attacker.invalid"
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if err := json.Unmarshal(response.Body.Bytes(), &manifest); err != nil || manifest.SourceURL != session.URL+"/source" {
		t.Fatalf("untrusted Host changed source URL: %#v, %v", manifest, err)
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
		return Source{Kind: "preset", ID: "preset-1", Name: "Preset", Filename: "preset.json", Path: path, Size: 2, SHA256: sourceHash([]byte(`{}`))}, nil
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

func TestChangedSourceIsRejectedBeforeDownload(t *testing.T) {
	path := t.TempDir() + "/card.json"
	raw := []byte(`{"name":"Original"}`)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	server := NewServer(func(context.Context, string, string) (Source, error) {
		return Source{Kind: "character", ID: "card-1", Name: "Original", Filename: "card.json", Path: path, SHA256: sourceHash(raw)}, nil
	})
	t.Cleanup(func() { _ = server.Close() })
	session, err := server.Create(context.Background(), "character", "card-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"name":"Modified"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/v1/transfers/"+session.ID+"/source", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusGone {
		t.Fatalf("changed source code = %d, want 410", response.Code)
	}
}

func TestAddressCandidatesPreferPhysicalPrivateNetworks(t *testing.T) {
	candidates := rankAddressCandidates([]addressCandidate{
		{host: "10.0.0.4", interfaceName: "Tailscale", virtual: true, preferred: true},
		{host: "192.168.1.20", interfaceName: "Wi-Fi"},
		{host: "172.20.0.1", interfaceName: "vEthernet", virtual: true},
	})
	if len(candidates) != 3 || candidates[0].host != "192.168.1.20" {
		t.Fatalf("unexpected address order: %#v", candidates)
	}
	for _, value := range []string{"10.1.2.3", "172.16.0.1", "172.31.255.254", "192.168.0.1"} {
		if !isPrivateIPv4(net.ParseIP(value)) {
			t.Fatalf("private address %s was rejected", value)
		}
	}
	for _, value := range []string{"8.8.8.8", "100.64.0.1", "169.254.1.1", "172.32.0.1"} {
		if isPrivateIPv4(net.ParseIP(value)) {
			t.Fatalf("non-private address %s was accepted", value)
		}
	}
}

func sourceHash(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
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
