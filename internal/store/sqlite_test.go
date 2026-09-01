package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/openai/tavern-shelf/internal/library"
	"github.com/openai/tavern-shelf/internal/manifest"
	_ "modernc.org/sqlite"
)

func TestOpenMigratesLibraryWithoutManifestColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	const oldSchema = `CREATE TABLE characters (
id TEXT PRIMARY KEY, source_hash TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
creator TEXT NOT NULL DEFAULT '', spec TEXT NOT NULL DEFAULT '', spec_version TEXT NOT NULL DEFAULT '',
tags_json TEXT NOT NULL DEFAULT '[]', has_worldbook INTEGER NOT NULL DEFAULT 0,
has_regex INTEGER NOT NULL DEFAULT 0, has_extensions INTEGER NOT NULL DEFAULT 0,
has_interactive INTEGER NOT NULL DEFAULT 0, source_format TEXT NOT NULL,
source_is_image INTEGER NOT NULL DEFAULT 0, source_filename TEXT NOT NULL,
source_rel_path TEXT NOT NULL, source_size INTEGER NOT NULL, imported_at TEXT NOT NULL
)`
	if _, err := db.Exec(oldSchema); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	character := library.Character{
		ID: "hash", SourceHash: "hash", Name: "Migrated", Tags: []string{}, SourceFormat: "json",
		SourceFilename: "card.json", SourceRelPath: "ha/hash/source.json", ImportedAt: time.Now(),
		Manifest: manifest.Content{SchemaVersion: manifest.CurrentSchemaVersion, Character: manifest.Character{Name: "Migrated"}},
	}
	if err := s.Create(context.Background(), character); err != nil {
		t.Fatal(err)
	}
	stored, err := s.Get(context.Background(), character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Manifest.Character.Name != "Migrated" {
		t.Fatalf("manifest was not stored after migration: %#v", stored.Manifest)
	}
}
