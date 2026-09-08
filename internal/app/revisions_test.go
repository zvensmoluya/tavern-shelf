package app

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zvensmoluya/tavern-shelf/internal/library"
)

func TestRevisionBackupTrashAndMigration(t *testing.T) {
	ctx := context.Background()
	source, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	raw := `{"spec":"chara_card_v2","data":{"name":"Mara","creator":"Ink","character_version":"1.0","first_mes":"Hello"}}`
	add := func(raw string) library.Character {
		t.Helper()
		r, e := source.ImportUpload(ctx, "Mara.json", strings.NewReader(raw))
		if e != nil {
			t.Fatal(e)
		}
		return r.Character
	}
	first := add(raw)
	second := add(strings.Replace(raw, "Hello", "Hello again", 1))
	third := add(strings.Replace(raw, "Hello", "Hello third", 1))
	if err := source.Store.ChangeAssociation(ctx, third.ID, "split", ""); err != nil {
		t.Fatal(err)
	}
	collection, err := source.CreateCollection(ctx, "Revisions")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{first.ID, second.ID} {
		if err := source.OrganizeCharacter(ctx, id, library.CharacterOrganization{CollectionIDs: []string{collection.ID}}); err != nil {
			t.Fatal(err)
		}
	}
	collections, err := source.Collections(ctx)
	if err != nil || len(collections) != 1 || collections[0].CharacterCount != 2 {
		t.Fatal("collection count must preserve independent cards")
	}
	var archive bytes.Buffer
	if _, err := source.WriteBackup(ctx, &archive); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	summary, err := restored.RestoreBackup(ctx, bytes.NewReader(archive.Bytes()))
	if err != nil || summary.Failed != 0 {
		t.Fatalf("restore: %+v %v", summary, err)
	}
	a, _ := restored.Get(ctx, first.ID)
	b, _ := restored.Get(ctx, second.ID)
	c, _ := restored.Get(ctx, third.ID)
	if a.GroupID != b.GroupID || c.GroupID == a.GroupID {
		t.Fatal("backup lost associations")
	}
	if a.ContentHash != first.ContentHash || !a.ImportedAt.Equal(first.ImportedAt) {
		t.Fatal("backup changed content or collection time")
	}
	// Repeating a restore must preserve local regrouping choices.
	if err := restored.Store.ChangeAssociation(ctx, second.ID, "split", ""); err != nil {
		t.Fatal(err)
	}
	before, _ := restored.Get(ctx, second.ID)
	if _, err := restored.RestoreBackup(ctx, bytes.NewReader(archive.Bytes())); err != nil {
		t.Fatal(err)
	}
	after, _ := restored.Get(ctx, second.ID)
	if after.GroupID != before.GroupID {
		t.Fatal("duplicate restore overwrote local choice")
	}
	if err := source.Delete(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	trash, err := source.ListTrash()
	if err != nil || len(trash) != 1 {
		t.Fatalf("trash: %v", err)
	}
	if _, err := source.RestoreTrash(ctx, trash[0].ID); err != nil {
		t.Fatal(err)
	}
	recovered, _ := source.Get(ctx, second.ID)
	if recovered.GroupID != first.GroupID || !recovered.ImportedAt.Equal(second.ImportedAt) {
		t.Fatal("trash restore lost revision metadata")
	}
}

func TestExistingLibraryRebuildsRevisionIdentityWithoutChangingSources(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	shelf, err := Open(Options{DataDir: root})
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"spec":"chara_card_v2","data":{"name":"Legacy","creator":"Writer","first_mes":"Hello"}}`
	first, err := shelf.ImportUpload(ctx, "first.json", strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	second, err := shelf.ImportUpload(ctx, "second.json", strings.NewReader(strings.Replace(raw, "Hello", "Again", 1)))
	if err != nil {
		t.Fatal(err)
	}
	database := shelf.Paths.Database
	if err := shelf.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(database))
	if err != nil {
		t.Fatal(err)
	}
	// Simulate rows created before the grouping migration, with valid manifests.
	if _, err := db.Exec(`UPDATE characters SET group_id='', group_reason='', content_hash='', identity_key='', cover_hash=''`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(Options{DataDir: root})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	a, err := reopened.Get(ctx, first.Character.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := reopened.Get(ctx, second.Character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if a.ContentHash == "" || b.ContentHash == "" || a.GroupID != b.GroupID || a.ContentHash == b.ContentHash {
		t.Fatal("old library identity was not safely rebuilt")
	}
	if !a.ImportedAt.Equal(first.Character.ImportedAt) {
		t.Fatal("migration changed collection time")
	}
}
