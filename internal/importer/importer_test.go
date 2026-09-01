package importer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/openai/tavern-shelf/internal/paths"
	"github.com/openai/tavern-shelf/internal/store"
)

const validCard = `{"spec":"chara_card_v2","spec_version":"2.0","data":{"name":"Mara","creator":"Inkkeeper","tags":["fantasy"]}}`

func TestImportCopiesThenRemovesInboxSource(t *testing.T) {
	imp, p, s := testImporter(t)
	source := filepath.Join(p.Inbox, "mara.json")
	if err := os.WriteFile(source, []byte(validCard), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := imp.Import(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if result.Duplicate || result.Character.Name != "Mara" {
		t.Fatalf("unexpected import result: %#v", result)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("Inbox source still exists after committed import: %v", err)
	}
	managed := filepath.Join(p.Library, result.Character.SourceRelPath)
	if raw, err := os.ReadFile(managed); err != nil || string(raw) != validCard {
		t.Fatalf("managed source mismatch: %q, %v", raw, err)
	}
	characters, err := s.List(context.Background())
	if err != nil || len(characters) != 1 {
		t.Fatalf("library mismatch: %#v, %v", characters, err)
	}
}

func TestDuplicateIsArchivedWithoutCreatingAnotherCharacter(t *testing.T) {
	imp, p, s := testImporter(t)
	first := filepath.Join(p.Inbox, "first.json")
	if err := os.WriteFile(first, []byte(validCard), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := imp.Import(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	duplicate := filepath.Join(p.Inbox, "again.json")
	if err := os.WriteFile(duplicate, []byte(validCard), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := imp.Import(context.Background(), duplicate)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Duplicate {
		t.Fatal("expected duplicate result")
	}
	characters, _ := s.List(context.Background())
	if len(characters) != 1 {
		t.Fatalf("duplicate created another row: %d", len(characters))
	}
	duplicates, err := os.ReadDir(p.Duplicate)
	if err != nil || len(duplicates) != 1 {
		t.Fatalf("duplicate was not archived: %v, %v", duplicates, err)
	}
}

func TestInvalidCardRemainsInInbox(t *testing.T) {
	imp, p, _ := testImporter(t)
	source := filepath.Join(p.Inbox, "broken.json")
	if err := os.WriteFile(source, []byte(`{"data":`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := imp.Import(context.Background(), source); err == nil {
		t.Fatal("expected invalid card to fail")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("invalid source was not preserved: %v", err)
	}
}

func TestImportFromConfiguredExternalInbox(t *testing.T) {
	imp, p, _ := testImporter(t)
	external := t.TempDir()
	source := filepath.Join(external, "external.json")
	if err := os.WriteFile(source, []byte(validCard), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := imp.ImportFrom(context.Background(), external, source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("external Inbox source still exists after import: %v", err)
	}
	if _, err := os.Stat(filepath.Join(p.Library, result.Character.SourceRelPath)); err != nil {
		t.Fatalf("managed source missing: %v", err)
	}
}

func TestImportFromRejectsFileOutsideConfiguredInbox(t *testing.T) {
	imp, _, _ := testImporter(t)
	external := t.TempDir()
	other := t.TempDir()
	source := filepath.Join(other, "outside.json")
	if err := os.WriteFile(source, []byte(validCard), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := imp.ImportFrom(context.Background(), external, source); err == nil {
		t.Fatal("expected source outside configured Inbox to be rejected")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("rejected source was changed: %v", err)
	}
}

func testImporter(t *testing.T) (*Importer, paths.Paths, *store.Store) {
	t.Helper()
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
	return New(p, s), p, s
}
