//go:build windows

package paths

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsShortAndLongPathsHaveSameIdentity(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "Inbox")
	if err := windows.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	short := windowsShortPath(t, root)
	long, err := Canonical(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.EqualFold(short, long) {
		t.Skip("8.3 short names are unavailable on this volume")
	}
	if !Same(short, long) {
		t.Fatalf("short path %q and long path %q should have the same identity", short, long)
	}
	if !IsSameOrWithin(short, child) {
		t.Fatalf("child %q should be within short-form parent %q", child, short)
	}
	missingShort := filepath.Join(short, "Removed Inbox")
	missingLong := filepath.Join(long, "Removed Inbox")
	if !Same(missingShort, missingLong) {
		t.Fatalf("missing paths %q and %q should retain their parent identity", missingShort, missingLong)
	}
}

func windowsShortPath(t *testing.T, path string) string {
	t.Helper()
	input, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]uint16, windows.MAX_PATH)
	n, err := windows.GetShortPathName(input, &buffer[0], uint32(len(buffer)))
	if err != nil {
		t.Skipf("cannot obtain an 8.3 short path: %v", err)
	}
	if n == 0 || n >= uint32(len(buffer)) {
		t.Skip("cannot obtain an 8.3 short path")
	}
	return windows.UTF16ToString(buffer[:n])
}
