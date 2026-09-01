package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/openai/tavern-shelf/internal/card"
	"github.com/openai/tavern-shelf/internal/importer"
	"github.com/openai/tavern-shelf/internal/library"
	"github.com/openai/tavern-shelf/internal/paths"
	"github.com/openai/tavern-shelf/internal/scanner"
	"github.com/openai/tavern-shelf/internal/store"
)

type App struct {
	Paths   paths.Paths
	Store   *store.Store
	Scanner *scanner.Scanner
	logger  *slog.Logger
}

type Options struct {
	DataDir      string
	ScanInterval time.Duration
	StableFor    time.Duration
	RetryAfter   time.Duration
	Logger       *slog.Logger
	OnLibraryHit func()
}

func Open(options Options) (*App, error) {
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	p, err := paths.New(options.DataDir)
	if err != nil {
		return nil, err
	}
	if err := p.Ensure(); err != nil {
		return nil, err
	}
	s, err := store.Open(p.Database)
	if err != nil {
		return nil, err
	}
	imp := importer.New(p, s)
	scan := scanner.New(scanner.Config{
		Inbox: p.Inbox, Interval: options.ScanInterval, StableFor: options.StableFor,
		RetryAfter: options.RetryAfter, Import: imp, Logger: options.Logger,
		OnLibraryHit: options.OnLibraryHit,
	})
	result := &App{Paths: p, Store: s, Scanner: scan, logger: options.Logger}
	if err := result.backfillManifests(context.Background()); err != nil {
		_ = s.Close()
		return nil, err
	}
	return result, nil
}

func (a *App) Close() error { return a.Store.Close() }

func (a *App) List(ctx context.Context) ([]library.Character, error) {
	characters, err := a.Store.List(ctx)
	if err != nil {
		return nil, err
	}
	for index := range characters {
		enrich(&characters[index])
	}
	return characters, nil
}

func (a *App) Get(ctx context.Context, id string) (library.Character, error) {
	character, err := a.Store.Get(ctx, id)
	if err != nil {
		return library.Character{}, err
	}
	enrich(&character)
	return character, nil
}

func (a *App) SourcePath(ctx context.Context, id string) (library.Character, string, error) {
	character, err := a.Store.Get(ctx, id)
	if err != nil {
		return library.Character{}, "", err
	}
	path := filepath.Join(a.Paths.Library, character.SourceRelPath)
	if !isWithin(a.Paths.Library, path) {
		return library.Character{}, "", errors.New("stored source path escapes the managed Library")
	}
	return character, path, nil
}

// Delete moves a character's managed source to Shelf's Trash before removing
// its database row. Nothing outside the managed Library can be touched.
func (a *App) Delete(ctx context.Context, id string) error {
	character, source, err := a.SourcePath(ctx, id)
	if err != nil {
		return err
	}
	managedDir := filepath.Dir(source)
	if filepath.Base(managedDir) != character.ID || !isWithin(a.Paths.Library, managedDir) {
		return errors.New("refusing to delete an unexpected managed source path")
	}
	trashDir := filepath.Join(a.Paths.Trash, fmt.Sprintf("%s-%d", character.ID, time.Now().UnixNano()))
	if err := os.Rename(managedDir, trashDir); err != nil {
		return fmt.Errorf("move character to Shelf Trash: %w", err)
	}
	if err := a.Store.Delete(ctx, id); err != nil {
		if restoreErr := os.Rename(trashDir, managedDir); restoreErr != nil {
			return fmt.Errorf("delete database row: %v; restore source: %w", err, restoreErr)
		}
		return err
	}
	return nil
}

func enrich(character *library.Character) {
	character.SourceURL = "/api/characters/" + character.ID + "/source"
	if character.SourceIsImage {
		character.AvatarURL = character.SourceURL
	}
}

func (a *App) backfillManifests(ctx context.Context) error {
	characters, err := a.Store.List(ctx)
	if err != nil {
		return fmt.Errorf("list characters for content manifest migration: %w", err)
	}
	for _, character := range characters {
		if !character.Manifest.Empty() {
			continue
		}
		path := filepath.Join(a.Paths.Library, character.SourceRelPath)
		if !isWithin(a.Paths.Library, path) {
			a.logger.Warn("skip manifest migration for unsafe source path", "character", character.ID)
			continue
		}
		parsed, err := card.ParseFile(path)
		if err != nil {
			a.logger.Warn("could not rebuild character content manifest", "character", character.ID, "error", err)
			continue
		}
		if err := a.Store.UpdateParsed(ctx, library.Character{
			ID: character.ID, Name: parsed.Name, Creator: parsed.Creator,
			Spec: parsed.Spec, SpecVersion: parsed.SpecVersion, Tags: parsed.Tags,
			HasWorldbook: parsed.HasWorldbook, HasRegex: parsed.HasRegex,
			HasExtensions: parsed.HasExtensions, HasInteractive: parsed.HasInteractive,
			SourceFormat: parsed.SourceFormat, SourceIsImage: parsed.SourceIsImage,
			Manifest: parsed.Manifest,
		}); err != nil {
			return fmt.Errorf("save rebuilt content manifest for %q: %w", character.ID, err)
		}
	}
	return nil
}

func isWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !filepath.IsAbs(rel) && len(rel) > 0 && rel[:1] != "."
}
