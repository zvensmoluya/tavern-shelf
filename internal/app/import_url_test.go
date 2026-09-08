package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveCommunityImportURL(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"https://cdn.discordapp.com/attachments/1/2/card.png?ex=abc&is=def&hm=signature", "https://cdn.discordapp.com/attachments/1/2/card.png?ex=abc&is=def&hm=signature"},
		{"https://media.discordapp.net/attachments/1/2/card.png?ex=abc&is=def&hm=signature&format=webp&width=400&height=600", "https://cdn.discordapp.com/attachments/1/2/card.png?ex=abc&is=def&hm=signature"},
		{"https://chub.ai/characters/author/card?x=1", "https://api.chub.ai/api/characters/author/card?full=true"},
		{"author/card", "https://api.chub.ai/api/characters/author/card?full=true"},
		{"lorebooks/author/book", "https://api.chub.ai/api/lorebooks/author/book?full=true"},
		{"https://realm.risuai.net/character/abcd-1234", "https://realm.risuai.net/api/v1/download/png-v3/abcd-1234?non_commercial=true"},
		{"AICC/author/card", "https://aicharactercards.com/wp-json/pngapi/v1/image/author/card"},
		{"a7ca95a1-0c88-4e23-91b3-149db1e78ab9", "https://server.pygmalion.chat/api/export/character/a7ca95a1-0c88-4e23-91b3-149db1e78ab9/v2"},
		{"https://chub.ai.evil.test/characters/author/card", "https://chub.ai.evil.test/characters/author/card"},
	} {
		got, err := resolveImportURL(tc.input)
		if err != nil || got.url != tc.want {
			t.Errorf("resolve %q = %q, %v", tc.input, got.url, err)
		}
	}
	for _, input := range []string{"", "file:///secret.png", "https://user:password@example.com/card", "https://chub.ai/characters/../card", "not a link"} {
		if _, err := resolveImportURL(input); err == nil {
			t.Errorf("accepted invalid URL %q", input)
		}
	}
}

func TestURLImportStoresExactBytesAndDeduplicates(t *testing.T) {
	shelf, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer shelf.Close()
	signedQuery := "ex=abc&is=def&hm=secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != signedQuery {
			t.Error("signed query changed")
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("unexpected credentials")
		}
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/download?"+signedQuery, http.StatusFound)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''Original%20Card.json`)
		io.WriteString(w, testCharacterJSON)
	}))
	defer server.Close()
	for attempt := 0; attempt < 2; attempt++ {
		result, err := shelf.importURL(context.Background(), server.URL+"/redirect?"+signedQuery, server.Client())
		if err != nil {
			t.Fatal(err)
		}
		if result.Duplicate != (attempt == 1) || result.Character.SourceFilename != "Original Card.json" {
			t.Fatalf("unexpected result: %#v", result)
		}
		raw, err := os.ReadFile(filepath.Join(shelf.Paths.Library, result.Character.SourceRelPath))
		if err != nil || string(raw) != testCharacterJSON {
			t.Fatal("source bytes changed", err)
		}
	}
	entries, _ := os.ReadDir(shelf.Paths.Staging)
	if len(entries) != 0 {
		t.Fatal("staging files leaked")
	}
}

func TestURLImportFailuresLeaveLibraryEmpty(t *testing.T) {
	for _, mode := range []string{"html", "ordinary image", "forbidden", "too large", "truncated", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			shelf, err := Open(Options{DataDir: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			defer shelf.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "html":
					io.WriteString(w, "<!doctype html><title>Sign in</title>")
				case "ordinary image":
					w.Write([]byte("\x89PNG\r\n\x1a\n"))
				case "forbidden":
					w.WriteHeader(403)
				case "too large":
					w.Header().Set("Content-Length", "67108865")
				case "truncated":
					w.Header().Set("Content-Length", "500")
					io.WriteString(w, testCharacterJSON)
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancelled" {
				cancel()
			}
			_, err = shelf.importURL(ctx, server.URL+"/download?hm=private-signature", server.Client())
			if err == nil || strings.Contains(err.Error(), "private-signature") {
				t.Fatalf("invalid failure: %v", err)
			}
			cards, _ := shelf.Store.List(context.Background())
			if len(cards) != 0 {
				t.Fatal("failure committed a card")
			}
			entries, _ := os.ReadDir(shelf.Paths.Staging)
			if len(entries) != 0 {
				t.Fatal("failure left staging files")
			}
		})
	}
}

type importTransport func(*http.Request) (*http.Response, error)

func (f importTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCommunityExportPreservesUnknownFields(t *testing.T) {
	for _, input := range []string{"author/card", "lorebooks/author/book", "a7ca95a1-0c88-4e23-91b3-149db1e78ab9"} {
		t.Run(input, func(t *testing.T) {
			shelf, err := Open(Options{DataDir: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			defer shelf.Close()
			raw := []byte(`{"spec":"chara_card_v3","data":{"name":"Original","extensions":{"unknown":{"keep":true}}}}`)
			if strings.HasPrefix(input, "lorebooks/") {
				raw = []byte(`{"name":"Book","entries":[],"unknown":true}`)
			}
			client := &http.Client{Transport: importTransport(func(r *http.Request) (*http.Response, error) {
				body := raw
				if strings.Contains(r.URL.Path, "/api/characters/") || strings.Contains(r.URL.Path, "/api/lorebooks/") {
					body = []byte(`{"node":{"id":123,"max_res_url":"https://cdn.example.test/preview"}}`)
				}
				if r.URL.Host == "cdn.example.test" {
					body = []byte("preview is not a card")
				}
				if r.URL.Host == "server.pygmalion.chat" {
					body = append(append([]byte(`{"character":`), raw...), '}')
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body)), Request: r}, nil
			})}
			result, err := shelf.importURL(context.Background(), input, client)
			if err != nil {
				t.Fatal(err)
			}
			rel := result.Character.SourceRelPath
			if result.Resource.ID != "" {
				rel = result.Resource.SourceRelPath
			}
			stored, err := os.ReadFile(filepath.Join(shelf.Paths.Library, rel))
			if err != nil || !bytes.Equal(stored, raw) {
				t.Fatal("export was rewritten", err)
			}
		})
	}
}

func TestURLImportBlocksInternalDestinations(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "::1", "::ffff:127.0.0.1", "10.0.0.1", "169.254.169.254", "100.100.100.200", "fc00::1", "0.0.0.0"} {
		if publicImportIP(netip.MustParseAddr(address)) {
			t.Errorf("allowed internal IP %s", address)
		}
	}
	client := newImportClient()
	defer client.CloseIdleConnections()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("internal destination was reached") }))
	defer server.Close()
	_, _, err := fetchImport(context.Background(), client, server.URL+"/card?hm=secret")
	if err == nil || strings.Contains(err.Error(), "hm=secret") {
		t.Fatalf("internal URL failure = %v", err)
	}
	for _, address := range []string{"file:///secret", "http://user:password@public.test/"} {
		r, _ := http.NewRequest("GET", address, nil)
		if client.CheckRedirect(r, nil) == nil {
			t.Errorf("redirect allowed %s", address)
		}
	}
}

func TestLiveURLImport(t *testing.T) {
	input := os.Getenv("SHELF_TEST_IMPORT_URL")
	if input == "" {
		t.Skip("optional live download")
	}
	shelf, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer shelf.Close()
	result, err := shelf.ImportURL(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "character" {
		t.Fatal("not a character")
	}
	if want := os.Getenv("SHELF_TEST_IMPORT_HASH"); want != "" && result.Character.SourceHash != want {
		t.Fatal("download differs from expected original")
	}
}

// Keep bounded readers honest even when Content-Length is absent.
func TestFetchURLSizeWithoutLength(t *testing.T) {
	client := &http.Client{Transport: importTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(io.LimitReader(zeroReader{}, MaxUploadSize+1)), Request: r}, nil
	})}
	_, _, err := fetchImport(context.Background(), client, "https://example.test/file")
	if !errors.Is(err, ErrUploadTooLarge) {
		t.Fatalf("size limit = %v", err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }
