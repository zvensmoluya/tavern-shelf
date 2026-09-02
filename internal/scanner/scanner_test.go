package scanner

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zvensmoluya/tavern-shelf/internal/importer"
	"github.com/zvensmoluya/tavern-shelf/internal/paths"
	"github.com/zvensmoluya/tavern-shelf/internal/store"
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
	failures := scan.Status().Failures
	if len(failures) != 1 || failures[0].File != "broken.json" || failures[0].Error == "" || !failures[0].NextRetryAt.Equal(t0.Add(2*time.Second).Add(time.Hour).UTC()) {
		t.Fatalf("unexpected visible import failure: %#v", failures)
	}
	_ = scan.ScanOnce(context.Background(), t0.Add(3*time.Second))
	if !scan.Status().LastErrorAt.Equal(firstError) {
		t.Fatal("invalid file was retried before cooldown elapsed")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("invalid source was removed: %v", err)
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	_ = scan.ScanOnce(context.Background(), t0.Add(4*time.Second))
	if failures := scan.Status().Failures; len(failures) != 0 {
		t.Fatalf("removed file remains in visible failures: %#v", failures)
	}
}

func TestScannerImportsFromMultipleInboxes(t *testing.T) {
	p, _ := paths.New(t.TempDir())
	_ = p.Ensure()
	s, err := store.Open(p.Database)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	second := t.TempDir()
	scan := New(Config{
		Inboxes: []string{p.Inbox, second}, ManagedInbox: p.Inbox, StableFor: time.Second, RetryAfter: time.Hour,
		Import: importer.New(p, s), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	cards := map[string]string{
		filepath.Join(p.Inbox, "first.json"): `{"spec":"chara_card_v2","data":{"name":"First"}}`,
		filepath.Join(second, "second.json"): `{"spec":"chara_card_v2","data":{"name":"Second"}}`,
	}
	for path, raw := range cards {
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t0 := time.Now()
	if err := scan.ScanOnce(context.Background(), t0); err != nil {
		t.Fatal(err)
	}
	if err := scan.ScanOnce(context.Background(), t0.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	characters, err := s.List(context.Background())
	if err != nil || len(characters) != 2 {
		t.Fatalf("multiple Inbox import mismatch: %#v, %v", characters, err)
	}
	if _, err := os.Stat(filepath.Join(p.Inbox, "first.json")); !os.IsNotExist(err) {
		t.Fatalf("Shelf-owned Inbox source was not moved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(second, "second.json")); err != nil {
		t.Fatalf("external scan source was removed: %v", err)
	}
	if err := scan.ScanOnce(context.Background(), t0.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	characters, err = s.List(context.Background())
	if err != nil || len(characters) != 2 || scan.Status().Pending != 0 {
		t.Fatalf("unchanged external source was reprocessed: %#v, status=%#v, err=%v", characters, scan.Status(), err)
	}
}

func TestScannerImportsWorldbookAndPreset(t *testing.T) {
	p, _ := paths.New(t.TempDir())
	_ = p.Ensure()
	s, err := store.Open(p.Database)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	scan := New(Config{
		Inbox: p.Inbox, StableFor: time.Second, RetryAfter: time.Hour,
		Import: importer.New(p, s), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	files := map[string]string{
		"world.json":  `{"entries":{"0":{"key":["city"],"comment":"City","content":"A city.","disable":false}}}`,
		"preset.json": `{"name":"ChatML","input_sequence":"<user>","output_sequence":"<assistant>"}`,
	}
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(p.Inbox, name), []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t0 := time.Now()
	if err := scan.ScanOnce(context.Background(), t0); err != nil {
		t.Fatal(err)
	}
	if err := scan.ScanOnce(context.Background(), t0.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	resources, err := s.ListResources(context.Background(), "")
	if err != nil || len(resources) != 2 {
		t.Fatalf("resource scan mismatch: %#v, %v", resources, err)
	}
}
