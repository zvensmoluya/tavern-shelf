package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zvensmoluya/tavern-shelf/internal/app"
)

func TestLinkImportRequestValidation(t *testing.T) {
	shelf, err := app.Open(app.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer shelf.Close()
	handler, err := Handler(shelf)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		body, contentType, origin string
		status                    int
	}{
		{`{}`, "application/json", "", 400},
		{`{"url":"file:///secret.png"}`, "application/json", "", 400},
		{`{"url":"http://127.0.0.1/card.png"}`, "application/json", "", 400},
		{`{"url":"http://example.test/card.png"}`, "text/plain", "", 415},
		{`{"url":"http://example.test/card.png"}`, "application/json", "https://other.example", 403},
		{`{"url":"` + strings.Repeat("a", 22<<10) + `"}`, "application/json", "", 400},
	} {
		r := httptest.NewRequest(http.MethodPost, "/api/imports/url", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", tc.contentType)
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Errorf("status=%d want=%d body=%s", w.Code, tc.status, w.Body.String())
		}
	}
}
