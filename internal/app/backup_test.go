package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupRestoreMergesLibraryByContentHash(t *testing.T) {
	source, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	if _, err := source.ImportUpload(context.Background(), "Collected.json", bytes.NewBufferString(testCharacterJSON)); err != nil {
		t.Fatal(err)
	}
	worldbook := `{"entries":{"0":{"key":["city"],"comment":"City","content":"A city."}}}`
	if _, err := source.ImportUpload(context.Background(), "City.json", bytes.NewBufferString(worldbook)); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	summary, err := source.WriteBackup(context.Background(), &archive)
	if err != nil || summary.Items != 2 {
		t.Fatalf("write backup: %#v, %v", summary, err)
	}

	restored, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = restored.Close() })
	result, err := restored.RestoreBackup(context.Background(), bytes.NewReader(archive.Bytes()))
	if err != nil || result.Total != 2 || result.Imported != 2 || result.Failed != 0 {
		t.Fatalf("restore backup: %#v, %v", result, err)
	}
	characters, _ := restored.List(context.Background())
	resources, _ := restored.ListResources(context.Background(), "")
	if len(characters) != 1 || len(resources) != 1 || characters[0].SourceFilename != "Collected.json" || resources[0].SourceFilename != "City.json" {
		t.Fatalf("unexpected restored Library: characters=%#v resources=%#v", characters, resources)
	}
	sourceCharacters, _ := source.List(context.Background())
	if !characters[0].ImportedAt.Equal(sourceCharacters[0].ImportedAt) {
		t.Fatalf("restored import time = %v, want %v", characters[0].ImportedAt, sourceCharacters[0].ImportedAt)
	}
	result, err = restored.RestoreBackup(context.Background(), bytes.NewReader(archive.Bytes()))
	if err != nil || result.Imported != 0 || result.Duplicates != 2 {
		t.Fatalf("merge duplicate backup: %#v, %v", result, err)
	}
}

func TestRestoreRejectsBackupWithMismatchedHashBeforeImport(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	sourceEntry, _ := writer.Create("sources/character/not-the-hash.json")
	_, _ = sourceEntry.Write([]byte(testCharacterJSON))
	manifestEntry, _ := writer.Create("backup.json")
	_ = json.NewEncoder(manifestEntry).Encode(BackupManifest{
		Format: backupFormat, Version: backupVersion,
		Items: []BackupItem{{
			Kind: "character", Name: "Collected", SourceFilename: "Collected.json",
			SourceHash:  "0000000000000000000000000000000000000000000000000000000000000000",
			ArchivePath: "sources/character/not-the-hash.json",
		}},
	})
	_ = writer.Close()
	shelf, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	if _, err := shelf.RestoreBackup(context.Background(), bytes.NewReader(archive.Bytes())); err == nil {
		t.Fatal("expected mismatched backup hash to be rejected")
	}
	characters, _ := shelf.List(context.Background())
	if len(characters) != 0 {
		t.Fatalf("invalid backup changed Library: %#v", characters)
	}
}

func TestBackupRejectsManagedSourceWhoseContentChanged(t *testing.T) {
	shelf, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	result, err := shelf.ImportUpload(context.Background(), "Collected.json", bytes.NewBufferString(testCharacterJSON))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(shelf.Paths.Library, result.Character.SourceRelPath)
	if err := os.WriteFile(path, []byte(`{"spec":"chara_card_v2","data":{"name":"Changed"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := shelf.WriteBackup(context.Background(), &bytes.Buffer{}); err == nil {
		t.Fatal("expected changed managed source to fail backup integrity check")
	}
}

func TestTrashCanBeListedAndRestored(t *testing.T) {
	shelf, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	result, err := shelf.ImportUpload(context.Background(), "Collected.json", bytes.NewBufferString(testCharacterJSON))
	if err != nil {
		t.Fatal(err)
	}
	if err := shelf.Delete(context.Background(), result.Character.ID); err != nil {
		t.Fatal(err)
	}
	items, err := shelf.ListTrash()
	if err != nil || len(items) != 1 || items[0].Name != "Collected" || items[0].SourceFilename != "Collected.json" {
		t.Fatalf("unexpected Trash: %#v, %v", items, err)
	}
	summary, err := shelf.RestoreTrash(context.Background(), items[0].ID)
	if err != nil || summary.Imported != 1 {
		t.Fatalf("restore Trash: %#v, %v", summary, err)
	}
	character, err := shelf.Get(context.Background(), result.Character.ID)
	if err != nil || character.SourceFilename != "Collected.json" {
		t.Fatalf("restored character mismatch: %#v, %v", character, err)
	}
	items, err = shelf.ListTrash()
	if err != nil || len(items) != 0 {
		t.Fatalf("restored item remains in Trash: %#v, %v", items, err)
	}
}

func TestTrashResourceCanBeRestored(t *testing.T) {
	shelf, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	worldbook := `{"entries":{"0":{"key":["city"],"content":"A city."}}}`
	result, err := shelf.ImportUpload(context.Background(), "City.json", bytes.NewBufferString(worldbook))
	if err != nil {
		t.Fatal(err)
	}
	if err := shelf.DeleteResource(context.Background(), result.Resource.ID); err != nil {
		t.Fatal(err)
	}
	items, err := shelf.ListTrash()
	if err != nil || len(items) != 1 || items[0].Kind != "worldbook" {
		t.Fatalf("unexpected resource Trash: %#v, %v", items, err)
	}
	if _, err := shelf.RestoreTrash(context.Background(), items[0].ID); err != nil {
		t.Fatal(err)
	}
	resource, err := shelf.GetResource(context.Background(), result.Resource.ID)
	if err != nil || resource.SourceFilename != "City.json" {
		t.Fatalf("restored resource mismatch: %#v, %v", resource, err)
	}
}

func TestUnrecognizedTrashItemRemainsVisible(t *testing.T) {
	shelf, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	directory := filepath.Join(shelf.Paths.Trash, "damaged-item")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "notes.txt"), []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	items, err := shelf.ListTrash()
	if err != nil || len(items) != 1 || items[0].Kind != "unknown" || items[0].Error == "" {
		t.Fatalf("unrecognized Trash item was hidden: %#v, %v", items, err)
	}
}
