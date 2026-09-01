package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Paths contains every directory Tavern Shelf owns.
type Paths struct {
	Root      string `json:"root"`
	Inbox     string `json:"inbox"`
	Library   string `json:"library"`
	AppData   string `json:"appData"`
	Trash     string `json:"trash"`
	Database  string `json:"-"`
	Staging   string `json:"-"`
	Duplicate string `json:"-"`
}

func DefaultRoot() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user data directory: %w", err)
	}
	name := "tavern-shelf"
	if runtime.GOOS == "windows" {
		name = "Tavern Shelf"
	}
	return filepath.Join(base, name), nil
}

func New(root string) (Paths, error) {
	if root == "" {
		var err error
		root, err = DefaultRoot()
		if err != nil {
			return Paths{}, err
		}
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve data directory: %w", err)
	}
	p := Paths{
		Root:      abs,
		Inbox:     filepath.Join(abs, "Inbox"),
		Library:   filepath.Join(abs, "Library"),
		AppData:   filepath.Join(abs, "AppData"),
		Trash:     filepath.Join(abs, "Trash"),
		Database:  filepath.Join(abs, "AppData", "shelf.db"),
		Staging:   filepath.Join(abs, "AppData", "staging"),
		Duplicate: filepath.Join(abs, "AppData", "duplicates"),
	}
	return p, nil
}

func (p Paths) Ensure() error {
	for _, dir := range []string{p.Root, p.Inbox, p.Library, p.AppData, p.Trash, p.Staging, p.Duplicate} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create managed directory %q: %w", dir, err)
		}
	}
	return nil
}
