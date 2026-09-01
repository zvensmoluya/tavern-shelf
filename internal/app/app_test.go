package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openai/tavern-shelf/internal/library"
)

func TestDeleteOnlyMovesManagedCharacterToTrash(t *testing.T) {
	shelf, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	id := "abcdef0123456789"
	rel := filepath.Join("ab", id, "source.json")
	source := filepath.Join(shelf.Paths.Library, rel)
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte(`{"name":"Delete Me"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := shelf.Store.Create(context.Background(), library.Character{
		ID: id, SourceHash: id, Name: "Delete Me", Tags: []string{}, SourceFormat: "json",
		SourceFilename: "delete-me.json", SourceRelPath: rel, SourceSize: 20,
	}); err != nil {
		t.Fatal(err)
	}
	if err := shelf.Delete(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("managed source still exists: %v", err)
	}
	trash, err := os.ReadDir(shelf.Paths.Trash)
	if err != nil || len(trash) != 1 {
		t.Fatalf("source was not moved to Trash: %v, %v", trash, err)
	}
	if _, err := shelf.Get(context.Background(), id); err == nil {
		t.Fatal("deleted character is still in the database")
	}
}

func TestOpenRebuildsMissingContentManifest(t *testing.T) {
	root := t.TempDir()
	shelf, err := Open(Options{DataDir: root})
	if err != nil {
		t.Fatal(err)
	}
	id := "1234567890abcdef"
	rel := filepath.Join("12", id, "source.json")
	source := filepath.Join(shelf.Paths.Library, rel)
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{"spec":"chara_card_v2","spec_version":"2.0","data":{"name":"Reindexed","creator":"Archivist","description":"Recovered content.","first_mes":"Hello.","alternate_greetings":["Again."],"tags":[],"extensions":{}}}`
	if err := os.WriteFile(source, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := shelf.Store.Create(context.Background(), library.Character{
		ID: id, SourceHash: id, Name: "Reindexed", Tags: []string{}, SourceFormat: "json",
		SourceFilename: "reindexed.json", SourceRelPath: rel, SourceSize: int64(len(raw)), ImportedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := shelf.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(Options{DataDir: root})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	character, err := reopened.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if character.Manifest.Empty() || character.Manifest.Character.Description != "Recovered content." || character.Manifest.Greetings.TotalCount != 2 {
		t.Fatalf("manifest was not rebuilt: %#v", character.Manifest)
	}
}
