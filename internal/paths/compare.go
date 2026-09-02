package paths

import (
	"path/filepath"
	"runtime"
	"strings"
)

// Canonical resolves a path to the filesystem identity used for comparisons.
// On Windows, EvalSymlinks also expands legacy 8.3 short names such as
// ADMINI~1. If the leaf no longer exists, the nearest existing ancestor is
// resolved so a missing configured Inbox can still be identified and removed.
func Canonical(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	current := filepath.Clean(abs)
	var missing []string
	for {
		resolved, resolveErr := filepath.EvalSymlinks(current)
		if resolveErr == nil {
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return filepath.Clean(resolved), nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", resolveErr
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func Same(left, right string) bool {
	left, err := Canonical(left)
	if err != nil {
		return false
	}
	right, err = Canonical(right)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func IsWithin(parent, child string) bool {
	parent, child, ok := canonicalPair(parent, child)
	if !ok {
		return false
	}
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != "." && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func IsSameOrWithin(parent, child string) bool {
	parent, child, ok := canonicalPair(parent, child)
	if !ok {
		return false
	}
	if equalCanonical(parent, child) {
		return true
	}
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func canonicalPair(left, right string) (string, string, bool) {
	left, err := Canonical(left)
	if err != nil {
		return "", "", false
	}
	right, err = Canonical(right)
	if err != nil {
		return "", "", false
	}
	return left, right, true
}

func equalCanonical(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
