//go:build !windows

package importer

import (
	"fmt"
	"os"
	"path/filepath"
)

// A portable path-based unlink cannot exclude an uncooperative writer. Keep
// the Inbox original in the owned archive instead of risking lost late writes.
// The unique directory also prevents replacing an existing archived source.
func finishInboxSource(source, expectedHash, archive string) error {
	currentHash, err := hashFile(source)
	if err != nil {
		return fmt.Errorf("verify Inbox source before archiving: %w", err)
	}
	if currentHash != expectedHash {
		return fmt.Errorf("Inbox source changed after copying; new content was preserved for the next scan")
	}
	directory, err := os.MkdirTemp(archive, "imported-")
	if err != nil {
		return fmt.Errorf("create Inbox source archive: %w", err)
	}
	if err := os.Rename(source, filepath.Join(directory, filepath.Base(source))); err != nil {
		_ = os.Remove(directory) // Empty, newly created directory only.
		return fmt.Errorf("archive Inbox source: %w", err)
	}
	return nil
}
