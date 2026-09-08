//go:build windows

package importer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupPreservesSourceWithAnOpenWriter(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.json")
	if err := os.WriteFile(source, []byte(validCard), 0600); err != nil {
		t.Fatal(err)
	}
	hash, err := hashFile(source)
	if err != nil {
		t.Fatal(err)
	}
	writer, err := os.OpenFile(source, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if err := finishInboxSource(source, hash, ""); err == nil {
		t.Fatal("source with an active writer was deleted")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal("active writer's source was lost")
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := finishInboxSource(source, hash, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("unchanged source was not cleaned up: %v", err)
	}
}
