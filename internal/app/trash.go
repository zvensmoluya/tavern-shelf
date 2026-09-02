package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/openai/tavern-shelf/internal/card"
	resourceparser "github.com/openai/tavern-shelf/internal/resource"
)

const trashMetadataFilename = "trash.json"

type TrashItem struct {
	ID             string    `json:"id"`
	Kind           string    `json:"kind"`
	Name           string    `json:"name"`
	SourceFilename string    `json:"sourceFilename"`
	SourceSize     int64     `json:"sourceSize"`
	DeletedAt      time.Time `json:"deletedAt"`
	Error          string    `json:"error,omitempty"`
}

type trashMetadata struct {
	Kind           string    `json:"kind"`
	Name           string    `json:"name"`
	SourceFilename string    `json:"sourceFilename"`
	DeletedAt      time.Time `json:"deletedAt"`
}

func (a *App) ListTrash() ([]TrashItem, error) {
	entries, err := os.ReadDir(a.Paths.Trash)
	if err != nil {
		return nil, fmt.Errorf("read Shelf Trash: %w", err)
	}
	items := make([]TrashItem, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directory := filepath.Join(a.Paths.Trash, entry.Name())
		item, err := inspectTrashItem(entry.Name(), directory)
		if err != nil {
			info, _ := entry.Info()
			item = TrashItem{ID: entry.Name(), Kind: "unknown", Name: entry.Name(), Error: err.Error()}
			if info != nil {
				item.DeletedAt = info.ModTime().UTC()
			}
		}
		items = append(items, item)
	}
	sort.Slice(items, func(left, right int) bool { return items[left].DeletedAt.After(items[right].DeletedAt) })
	return items, nil
}

func (a *App) RestoreTrash(ctx context.Context, id string) (RestoreSummary, error) {
	if id == "" || filepath.Base(id) != id || strings.ContainsAny(id, `/\\`) {
		return RestoreSummary{}, errors.New("invalid Trash item ID")
	}
	directory := filepath.Join(a.Paths.Trash, id)
	if !isWithin(a.Paths.Trash, directory) {
		return RestoreSummary{}, errors.New("refusing to restore an unexpected Trash path")
	}
	item, err := inspectTrashItem(id, directory)
	if err != nil {
		return RestoreSummary{}, err
	}
	source, err := trashSource(directory)
	if err != nil {
		return RestoreSummary{}, err
	}
	file, err := os.Open(source)
	if err != nil {
		return RestoreSummary{}, fmt.Errorf("open Trash source: %w", err)
	}
	result, importErr := a.importReader(ctx, item.SourceFilename, file, maxBackupItemSize, errBackupItemTooLarge, "trash-restore-", false)
	closeErr := file.Close()
	if importErr != nil {
		return RestoreSummary{}, fmt.Errorf("restore Trash source: %w", importErr)
	}
	if closeErr != nil {
		return RestoreSummary{}, fmt.Errorf("close Trash source: %w", closeErr)
	}
	if err := os.RemoveAll(directory); err != nil {
		return RestoreSummary{}, fmt.Errorf("remove restored Trash item: %w", err)
	}
	summary := RestoreSummary{Total: 1}
	if result.Duplicate {
		summary.Duplicates = 1
	} else {
		summary.Imported = 1
	}
	return summary, nil
}

func writeTrashMetadata(directory string, metadata trashMetadata) error {
	raw, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Trash metadata: %w", err)
	}
	path := filepath.Join(directory, trashMetadataFilename)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write Trash metadata: %w", err)
	}
	return nil
}

func inspectTrashItem(id, directory string) (TrashItem, error) {
	source, err := trashSource(directory)
	if err != nil {
		return TrashItem{}, err
	}
	info, err := os.Stat(source)
	if err != nil {
		return TrashItem{}, fmt.Errorf("inspect Trash source: %w", err)
	}
	metadata := trashMetadata{SourceFilename: filepath.Base(source), DeletedAt: info.ModTime().UTC()}
	if raw, readErr := os.ReadFile(filepath.Join(directory, trashMetadataFilename)); readErr == nil {
		_ = json.Unmarshal(raw, &metadata)
	}
	if metadata.Kind == "" || metadata.Name == "" {
		if parsed, parseErr := card.ParseFile(source); parseErr == nil {
			metadata.Kind, metadata.Name = "character", parsed.Name
		} else if parsed, parseErr := resourceparser.ParseFile(source); parseErr == nil {
			metadata.Kind, metadata.Name = parsed.Kind, parsed.Name
		} else {
			return TrashItem{}, errors.New("Trash source is not a supported Shelf resource")
		}
	}
	if metadata.SourceFilename == "" {
		metadata.SourceFilename = filepath.Base(source)
	}
	return TrashItem{
		ID: id, Kind: metadata.Kind, Name: metadata.Name, SourceFilename: metadata.SourceFilename,
		SourceSize: info.Size(), DeletedAt: metadata.DeletedAt,
	}, nil
}

func trashSource(directory string) (string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", fmt.Errorf("read Trash item: %w", err)
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if extension == ".png" || extension == ".json" {
			return filepath.Join(directory, entry.Name()), nil
		}
	}
	return "", errors.New("Trash item has no supported source")
}
