package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
