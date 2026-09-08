//go:build windows

package importer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// Windows can deny concurrent writes and path replacement while we verify and
// delete this exact file handle. Rehashing followed by os.Remove would still
// leave a race between the check and deletion of the named path.
func finishInboxSource(source, expectedHash, _ string) error {
	name, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.DELETE,
		windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return fmt.Errorf("preserve Inbox source: cannot open for exclusive cleanup: %w", err)
	}
	file := os.NewFile(uintptr(handle), source)
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("preserve Inbox source: source is no longer a regular file")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("verify Inbox source before cleanup: %w", err)
	}
	if hex.EncodeToString(hash.Sum(nil)) != expectedHash {
		return errors.New("Inbox source changed after copying; new content was preserved for the next scan")
	}
	deleteFile := byte(1)
	if err := windows.SetFileInformationByHandle(handle, windows.FileDispositionInfo, &deleteFile, 1); err != nil {
		return fmt.Errorf("preserve Inbox source: cannot finish cleanup: %w", err)
	}
	return nil
}
