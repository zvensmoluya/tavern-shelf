package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/openai/tavern-shelf/internal/card"
	"github.com/openai/tavern-shelf/internal/importer"
	"github.com/openai/tavern-shelf/internal/library"
	"github.com/openai/tavern-shelf/internal/paths"
	"github.com/openai/tavern-shelf/internal/scanner"
	"github.com/openai/tavern-shelf/internal/store"
	"github.com/openai/tavern-shelf/internal/transfer"
)

type App struct {
	Paths        paths.Paths
	Store        *store.Store
	Scanner      *scanner.Scanner
	Importer     *importer.Importer
	Transfers    *transfer.Server
	logger       *slog.Logger
	inboxMu      sync.Mutex
	runCtx       context.Context
	cancel       context.CancelFunc
	workWG       sync.WaitGroup
	stableFor    time.Duration
	onLibraryHit func()
	scanMu       sync.RWMutex
	oneShotScan  OneShotScanStatus
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
	inboxes, err := s.InboxDirectories(context.Background())
	if err != nil {
		_ = s.Close()
		return nil, err
	}
	if len(inboxes) == 0 {
		inboxes = []string{p.Inbox}
		if err := s.ReplaceInboxDirectories(context.Background(), inboxes); err != nil {
			_ = s.Close()
			return nil, err
		}
	}
	if options.StableFor <= 0 {
		options.StableFor = 2 * time.Second
	}
	imp := importer.New(p, s)
	scan := scanner.New(scanner.Config{
		Inboxes: inboxes, ManagedInbox: p.Inbox, Interval: options.ScanInterval, StableFor: options.StableFor,
		RetryAfter: options.RetryAfter, Import: imp, Logger: options.Logger,
		OnLibraryHit: options.OnLibraryHit,
	})
	runCtx, cancel := context.WithCancel(context.Background())
	result := &App{
		Paths: p, Store: s, Scanner: scan, Importer: imp, logger: options.Logger,
		runCtx: runCtx, cancel: cancel, stableFor: options.StableFor, onLibraryHit: options.OnLibraryHit,
	}
	result.Transfers = transfer.NewServer(result.resolveTransferSource)
	if err := result.backfillManifests(context.Background()); err != nil {
		cancel()
		_ = s.Close()
		return nil, err
	}
	return result, nil
}

func (a *App) Close() error {
	a.cancel()
	a.workWG.Wait()
	transferErr := a.Transfers.Close()
	storeErr := a.Store.Close()
	if transferErr != nil {
		return transferErr
	}
	return storeErr
}

func (a *App) Inboxes() []string { return a.Scanner.Inboxes() }

func (a *App) InboxMode(directory string) string {
	if samePath(directory, a.Paths.Inbox) {
		return "move"
	}
	return "copy"
}

func (a *App) AddInbox(ctx context.Context, directory string) error {
	a.inboxMu.Lock()
	defer a.inboxMu.Unlock()
	directory, err := a.validateInbox(directory)
	if err != nil {
		return err
	}
	directories := a.Inboxes()
	for _, existing := range directories {
		if samePath(existing, directory) {
			return nil
		}
	}
	directories = append(directories, directory)
	if err := a.Store.ReplaceInboxDirectories(ctx, directories); err != nil {
		return err
	}
	a.Scanner.SetInboxes(directories)
	return nil
}

func (a *App) RemoveInbox(ctx context.Context, directory string) error {
	a.inboxMu.Lock()
	defer a.inboxMu.Unlock()
	directory, err := filepath.Abs(strings.TrimSpace(directory))
	if err != nil {
		return fmt.Errorf("resolve Inbox directory: %w", err)
	}
	directories := a.Inboxes()
	if len(directories) <= 1 {
		return errors.New("at least one Inbox directory is required")
	}
	next := make([]string, 0, len(directories)-1)
	found := false
	for _, existing := range directories {
		if samePath(existing, directory) {
			found = true
			continue
		}
		next = append(next, existing)
	}
	if !found {
		return errors.New("Inbox directory is not configured")
	}
	if err := a.Store.ReplaceInboxDirectories(ctx, next); err != nil {
		return err
	}
	a.Scanner.SetInboxes(next)
	return nil
}

func (a *App) HasInbox(directory string) bool {
	abs, err := filepath.Abs(strings.TrimSpace(directory))
	if err != nil {
		return false
	}
	for _, existing := range a.Inboxes() {
		if samePath(existing, abs) {
			return true
		}
	}
	return false
}

func (a *App) validateInbox(directory string) (string, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return "", errors.New("Inbox directory is required")
	}
	abs, err := filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("resolve Inbox directory: %w", err)
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve Inbox directory links: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("inspect Inbox directory: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("Inbox path must be a directory")
	}
	if samePath(abs, a.Paths.Root) || isSameOrWithin(a.Paths.Library, abs) || isSameOrWithin(a.Paths.AppData, abs) || isSameOrWithin(a.Paths.Trash, abs) {
		return "", errors.New("managed Library, AppData, and Trash paths cannot be used as Inbox directories")
	}
	return filepath.Clean(abs), nil
}

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

func (a *App) ListResources(ctx context.Context, kind string) ([]library.Resource, error) {
	resources, err := a.Store.ListResources(ctx, kind)
	if err != nil {
		return nil, err
	}
	for index := range resources {
		enrichResource(&resources[index])
	}
	return resources, nil
}

func (a *App) GetResource(ctx context.Context, id string) (library.Resource, error) {
	resource, err := a.Store.GetResource(ctx, id)
	if err != nil {
		return library.Resource{}, err
	}
	enrichResource(&resource)
	return resource, nil
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

func (a *App) ResourceSourcePath(ctx context.Context, id string) (library.Resource, string, error) {
	resource, err := a.Store.GetResource(ctx, id)
	if err != nil {
		return library.Resource{}, "", err
	}
	path := filepath.Join(a.Paths.Library, resource.SourceRelPath)
	if !isWithin(a.Paths.Library, path) {
		return library.Resource{}, "", errors.New("stored resource source path escapes the managed Library")
	}
	return resource, path, nil
}

func (a *App) resolveTransferSource(ctx context.Context, kind, id string) (transfer.Source, error) {
	switch kind {
	case "character":
		character, path, err := a.SourcePath(ctx, id)
		if err != nil {
			return transfer.Source{}, err
		}
		return transfer.Source{
			Kind: kind, ID: character.ID, Name: character.Name, Filename: character.SourceFilename,
			Path: path, Size: character.SourceSize, SHA256: character.SourceHash,
		}, nil
	case library.ResourceWorldbook, library.ResourcePreset:
		resource, path, err := a.ResourceSourcePath(ctx, id)
		if err != nil {
			return transfer.Source{}, err
		}
		if resource.Kind != kind {
			return transfer.Source{}, store.ErrNotFound
		}
		return transfer.Source{
			Kind: resource.Kind, ID: resource.ID, Name: resource.Name, Subtype: resource.Subtype,
			Filename: resource.SourceFilename, Path: path, Size: resource.SourceSize, SHA256: resource.SourceHash,
		}, nil
	default:
		return transfer.Source{}, errors.New("unsupported transfer resource kind")
	}
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

func (a *App) DeleteResource(ctx context.Context, id string) error {
	resource, source, err := a.ResourceSourcePath(ctx, id)
	if err != nil {
		return err
	}
	managedDir := filepath.Dir(source)
	if filepath.Base(managedDir) != resource.ID || !isWithin(a.Paths.Library, managedDir) {
		return errors.New("refusing to delete an unexpected managed resource path")
	}
	trashDir := filepath.Join(a.Paths.Trash, fmt.Sprintf("%s-%d", resource.ID, time.Now().UnixNano()))
	if err := os.Rename(managedDir, trashDir); err != nil {
		return fmt.Errorf("move resource to Shelf Trash: %w", err)
	}
	if err := a.Store.DeleteResource(ctx, id); err != nil {
		if restoreErr := os.Rename(trashDir, managedDir); restoreErr != nil {
			return fmt.Errorf("delete resource database row: %v; restore source: %w", err, restoreErr)
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

func enrichResource(resource *library.Resource) {
	resource.SourceURL = "/api/resources/" + resource.ID + "/source"
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

func isSameOrWithin(parent, child string) bool {
	if samePath(parent, child) {
		return true
	}
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func samePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
