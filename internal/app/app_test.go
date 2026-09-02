package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zvensmoluya/tavern-shelf/internal/library"
)

const testCharacterJSON = `{"spec":"chara_card_v2","spec_version":"2.0","data":{"name":"Collected","description":"Safe source","first_mes":"Hello"}}`

func TestImportUploadCopiesIntoManagedLibrary(t *testing.T) {
	shelf, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	result, err := shelf.ImportUpload(context.Background(), "Collected.json", bytes.NewBufferString(testCharacterJSON))
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "character" || result.Name != "Collected" || result.Duplicate {
		t.Fatalf("unexpected import result: %#v", result)
	}
	managed := filepath.Join(shelf.Paths.Library, result.Character.SourceRelPath)
	if raw, err := os.ReadFile(managed); err != nil || string(raw) != testCharacterJSON {
		t.Fatalf("managed source mismatch: %q, %v", raw, err)
	}
	duplicate, err := shelf.ImportUpload(context.Background(), "Collected again.json", bytes.NewBufferString(testCharacterJSON))
	if err != nil || !duplicate.Duplicate || duplicate.Character.ID != result.Character.ID {
		t.Fatalf("unexpected duplicate result: %#v, %v", duplicate, err)
	}
}

func TestCharacterOrganizationSurvivesTrashRestore(t *testing.T) {
	shelf, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	result, err := shelf.ImportUpload(context.Background(), "Collected.json", bytes.NewBufferString(testCharacterJSON))
	if err != nil {
		t.Fatal(err)
	}
	first, err := shelf.CreateCollection(context.Background(), "Favorites for RP")
	if err != nil {
		t.Fatal(err)
	}
	second, err := shelf.CreateCollection(context.Background(), "To Try")
	if err != nil {
		t.Fatal(err)
	}
	organization := library.CharacterOrganization{Favorite: true, Note: "Use the patient model.", CollectionIDs: []string{first.ID, second.ID}}
	if err := shelf.OrganizeCharacter(context.Background(), result.Character.ID, organization); err != nil {
		t.Fatal(err)
	}
	character, err := shelf.Get(context.Background(), result.Character.ID)
	if err != nil || !character.Favorite || character.Note != organization.Note || len(character.CollectionIDs) != 2 {
		t.Fatalf("unexpected character organization: %#v, %v", character, err)
	}
	if err := shelf.Delete(context.Background(), character.ID); err != nil {
		t.Fatal(err)
	}
	trash, err := shelf.ListTrash()
	if err != nil || len(trash) != 1 {
		t.Fatalf("unexpected Trash: %#v, %v", trash, err)
	}
	if _, err := shelf.RestoreTrash(context.Background(), trash[0].ID); err != nil {
		t.Fatal(err)
	}
	restored, err := shelf.Get(context.Background(), character.ID)
	if err != nil || !restored.Favorite || restored.Note != organization.Note || len(restored.CollectionIDs) != 2 {
		t.Fatalf("organization did not survive Trash restore: %#v, %v", restored, err)
	}
	if err := shelf.DeleteCollection(context.Background(), first.ID); err != nil {
		t.Fatal(err)
	}
	restored, err = shelf.Get(context.Background(), character.ID)
	if err != nil || len(restored.CollectionIDs) != 1 || restored.CollectionIDs[0] != second.ID {
		t.Fatalf("deleting collection changed character incorrectly: %#v, %v", restored, err)
	}
}

func TestOneShotScanCopiesWithoutRememberingDirectory(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	source := filepath.Join(external, "Collected.json")
	if err := os.WriteFile(source, []byte(testCharacterJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	shelf, err := Open(Options{DataDir: root, StableFor: 5 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	started, err := shelf.StartScanOnce(context.Background(), external)
	if err != nil || !started.Running || started.Total != 1 {
		t.Fatalf("unexpected started scan: %#v, %v", started, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for shelf.OneShotScanStatus().Running && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	finished := shelf.OneShotScanStatus()
	if finished.Running || finished.Imported != 1 || finished.Failed != 0 {
		t.Fatalf("unexpected finished scan: %#v", finished)
	}
	if raw, err := os.ReadFile(source); err != nil || string(raw) != testCharacterJSON {
		t.Fatalf("one-time scan changed source: %q, %v", raw, err)
	}
	if shelf.HasInbox(external) {
		t.Fatal("one-time scan directory was persisted as a watched Inbox")
	}
}

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

func TestInboxDirectoriesPersistAcrossRestart(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	shelf, err := Open(Options{DataDir: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := shelf.AddInbox(context.Background(), external); err != nil {
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
	directories := reopened.Inboxes()
	if len(directories) != 2 || directories[1] != external {
		t.Fatalf("Inbox settings were not restored: %#v", directories)
	}
	if reopened.InboxMode(directories[0]) != "move" || reopened.InboxMode(external) != "copy" {
		t.Fatalf("unexpected Inbox modes: default=%q external=%q", reopened.InboxMode(directories[0]), reopened.InboxMode(external))
	}
	if err := reopened.RemoveInbox(context.Background(), directories[0]); err != nil {
		t.Fatal(err)
	}
	if err := reopened.RemoveInbox(context.Background(), external); err == nil {
		t.Fatal("expected removing the final Inbox to fail")
	}
}

func TestAddInboxRejectsManagedDirectories(t *testing.T) {
	shelf, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	for _, directory := range []string{shelf.Paths.Root, shelf.Paths.Library, shelf.Paths.AppData, shelf.Paths.Trash} {
		if err := shelf.AddInbox(context.Background(), directory); err == nil {
			t.Fatalf("expected managed directory %q to be rejected", directory)
		}
	}
}

func TestRemoveInboxAfterDirectoryDisappears(t *testing.T) {
	external := t.TempDir()
	shelf, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	if err := shelf.AddInbox(context.Background(), external); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(external); err != nil {
		t.Fatal(err)
	}
	if err := shelf.RemoveInbox(context.Background(), external); err != nil {
		t.Fatalf("remove missing Inbox: %v", err)
	}
	if directories := shelf.Inboxes(); len(directories) != 1 || !samePath(directories[0], shelf.Paths.Inbox) {
		t.Fatalf("unexpected Inbox settings: %#v", directories)
	}
}

func TestDeleteResourceOnlyMovesManagedSourceToTrash(t *testing.T) {
	shelf, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	id := "feedface01234567"
	rel := filepath.Join("fe", id, "source.json")
	source := filepath.Join(shelf.Paths.Library, rel)
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte(`{"entries":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := shelf.Store.CreateResource(context.Background(), library.Resource{
		ID: id, SourceHash: id, Kind: library.ResourceWorldbook, Name: "Delete World",
		SourceFilename: "delete-world.json", SourceRelPath: rel, SourceSize: 14, ImportedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := shelf.DeleteResource(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("managed resource still exists: %v", err)
	}
	if _, err := shelf.GetResource(context.Background(), id); err == nil {
		t.Fatal("deleted resource is still in the database")
	}
}
