package scanner

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openai/tavern-shelf/internal/importer"
	"github.com/openai/tavern-shelf/internal/paths"
	"github.com/openai/tavern-shelf/internal/store"
)

func TestScannerWaitsForAStableFile(t *testing.T) {
	p, err := paths.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(p.Database)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	scan := New(Config{
		Inbox: p.Inbox, StableFor: time.Second, RetryAfter: time.Hour,
		Import: importer.New(p, s), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	source := filepath.Join(p.Inbox, "arriving.json")
	firstHalf := `{"spec":"chara_card_v2","data":{"name":"Arriving"`
	if err := os.WriteFile(source, []byte(firstHalf), 0o644); err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	if err := scan.ScanOnce(context.Background(), t0); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte(firstHalf+`}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := scan.ScanOnce(context.Background(), t0.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if characters, _ := s.List(context.Background()); len(characters) != 0 {
		t.Fatal("changed file was imported before being observed as stable")
	}
	if err := scan.ScanOnce(context.Background(), t0.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	characters, err := s.List(context.Background())
	if err != nil || len(characters) != 1 || characters[0].Name != "Arriving" {
		t.Fatalf("stable file was not imported: %#v, %v", characters, err)
	}
}

func TestScannerBacksOffInvalidFiles(t *testing.T) {
	p, _ := paths.New(t.TempDir())
	_ = p.Ensure()
	s, err := store.Open(p.Database)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	scan := New(Config{Inbox: p.Inbox, StableFor: time.Second, RetryAfter: time.Hour, Import: importer.New(p, s), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	source := filepath.Join(p.Inbox, "broken.json")
	if err := os.WriteFile(source, []byte(`{"data":`), 0o644); err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	_ = scan.ScanOnce(context.Background(), t0)
	_ = scan.ScanOnce(context.Background(), t0.Add(2*time.Second))
	firstError := scan.Status().LastErrorAt
	_ = scan.ScanOnce(context.Background(), t0.Add(3*time.Second))
	if !scan.Status().LastErrorAt.Equal(firstError) {
		t.Fatal("invalid file was retried before cooldown elapsed")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("invalid source was removed: %v", err)
	}
}
