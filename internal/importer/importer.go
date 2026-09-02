package importer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openai/tavern-shelf/internal/card"
	"github.com/openai/tavern-shelf/internal/library"
	"github.com/openai/tavern-shelf/internal/paths"
	resourceparser "github.com/openai/tavern-shelf/internal/resource"
	"github.com/openai/tavern-shelf/internal/store"
)

type Result struct {
	Character library.Character
	Resource  library.Resource
	Kind      string
	Name      string
	Duplicate bool
}

type Importer struct {
	paths paths.Paths
	store *store.Store
	now   func() time.Time
}

func New(p paths.Paths, s *store.Store) *Importer {
	return &Importer{paths: p, store: s, now: time.Now}
}

func (i *Importer) Import(ctx context.Context, source string) (Result, error) {
	return i.importFrom(ctx, i.paths.Inbox, source, true)
}

// ImportFrom imports from an external watched directory without changing the
// source file. Only Import, which is restricted to Shelf's owned Inbox, moves
// successfully handled sources into managed storage.
func (i *Importer) ImportFrom(ctx context.Context, inbox, source string) (Result, error) {
	return i.importFrom(ctx, inbox, source, false)
}

func (i *Importer) importFrom(ctx context.Context, inbox, source string, removeSource bool) (Result, error) {
	if err := ensureDirectChild(inbox, source); err != nil {
		return Result{}, err
	}
	info, err := os.Stat(source)
	if err != nil {
		return Result{}, fmt.Errorf("inspect inbox file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return Result{}, errors.New("inbox item is not a regular file")
	}
	if !card.Supported(source) {
		return Result{}, card.ErrUnsupported
	}
	isJSON := strings.ToLower(filepath.Ext(source)) == ".json"
	isCharacter := !isJSON
	var characterMetadata card.Character
	var resourceMetadata resourceparser.Parsed
	if isJSON {
		resourceMetadata, err = resourceparser.ParseFile(source)
		if err != nil {
			characterMetadata, err = card.ParseFile(source)
			if err != nil {
				return Result{}, fmt.Errorf("parse Inbox JSON as character, worldbook, or preset: %w", err)
			}
			isCharacter = true
		}
	} else {
		characterMetadata, err = card.ParseFile(source)
		if err != nil {
			return Result{}, err
		}
	}
	hash, err := hashFile(source)
	if err != nil {
		return Result{}, err
	}
	if existing, err := i.store.GetByHash(ctx, hash); err == nil {
		if removeSource {
			if err := i.archiveDuplicate(source, hash); err != nil {
				return Result{}, err
			}
		}
		return Result{Character: existing, Kind: "character", Name: existing.Name, Duplicate: true}, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return Result{}, err
	}
	if existing, err := i.store.GetResourceByHash(ctx, hash); err == nil {
		if removeSource {
			if err := i.archiveDuplicate(source, hash); err != nil {
				return Result{}, err
			}
		}
		return Result{Resource: existing, Kind: existing.Kind, Name: existing.Name, Duplicate: true}, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return Result{}, err
	}

	ext := strings.ToLower(filepath.Ext(source))
	relDir := filepath.Join(hash[:2], hash)
	relSource := filepath.Join(relDir, "source"+ext)
	finalDir := filepath.Join(i.paths.Library, relDir)
	finalSource := filepath.Join(i.paths.Library, relSource)
	stageDir, err := os.MkdirTemp(i.paths.Staging, "import-")
	if err != nil {
		return Result{}, fmt.Errorf("create import staging directory: %w", err)
	}
	defer os.RemoveAll(stageDir)
	stageSource := filepath.Join(stageDir, "source"+ext)
	stagedHash, err := copyAndHash(source, stageSource)
	if err != nil {
		return Result{}, err
	}
	if stagedHash != hash {
		return Result{}, errors.New("inbox file changed while it was being imported")
	}
	if err := os.MkdirAll(filepath.Dir(finalDir), 0o755); err != nil {
		return Result{}, fmt.Errorf("create library shard: %w", err)
	}
	if err := os.Rename(stageDir, finalDir); err != nil {
		return Result{}, fmt.Errorf("commit managed source: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(finalDir)
		}
	}()

	result := Result{}
	if isCharacter {
		character := library.Character{
			ID:             hash,
			SourceHash:     hash,
			Name:           characterMetadata.Name,
			Creator:        characterMetadata.Creator,
			Spec:           characterMetadata.Spec,
			SpecVersion:    characterMetadata.SpecVersion,
			Tags:           characterMetadata.Tags,
			HasWorldbook:   characterMetadata.HasWorldbook,
			HasRegex:       characterMetadata.HasRegex,
			HasExtensions:  characterMetadata.HasExtensions,
			HasInteractive: characterMetadata.HasInteractive,
			SourceFormat:   characterMetadata.SourceFormat,
			SourceIsImage:  characterMetadata.SourceIsImage,
			SourceFilename: filepath.Base(source),
			SourceRelPath:  relSource,
			SourceSize:     info.Size(),
			ImportedAt:     i.now().UTC(),
			Manifest:       characterMetadata.Manifest,
		}
		if err := i.store.Create(ctx, character); err != nil {
			return Result{}, err
		}
		result = Result{Character: character, Kind: "character", Name: character.Name}
	} else {
		resource := library.Resource{
			ID: hash, SourceHash: hash, Kind: resourceMetadata.Kind, Subtype: resourceMetadata.Subtype,
			Name: resourceMetadata.Name, Description: resourceMetadata.Description,
			SourceFilename: filepath.Base(source), SourceRelPath: relSource,
			SourceSize: info.Size(), ImportedAt: i.now().UTC(),
			Worldbook: resourceMetadata.Worldbook, Preset: resourceMetadata.Preset,
		}
		if err := i.store.CreateResource(ctx, resource); err != nil {
			return Result{}, err
		}
		result = Result{Resource: resource, Kind: resource.Kind, Name: resource.Name}
	}
	committed = true
	if removeSource {
		if err := os.Remove(source); err != nil {
			// The managed copy and database row are complete. A later scan will identify
			// this leftover as an exact duplicate and archive it safely.
			return result, fmt.Errorf("remove imported inbox file: %w", err)
		}
	}
	if _, err := os.Stat(finalSource); err != nil {
		return result, fmt.Errorf("verify managed source: %w", err)
	}
	return result, nil
}

func (i *Importer) archiveDuplicate(source, hash string) error {
	name := fmt.Sprintf("%s-%d%s", hash[:12], i.now().UnixNano(), strings.ToLower(filepath.Ext(source)))
	destination := filepath.Join(i.paths.Duplicate, name)
	if err := os.Rename(source, destination); err != nil {
		return fmt.Errorf("archive duplicate inbox file: %w", err)
	}
	return nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open source for hashing: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash source: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyAndHash(source, destination string) (string, error) {
	in, err := os.Open(source)
	if err != nil {
		return "", fmt.Errorf("open source for import: %w", err)
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create staged source: %w", err)
	}
	h := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(out, h), in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil {
		return "", fmt.Errorf("copy source to staging: %w", copyErr)
	}
	if syncErr != nil {
		return "", fmt.Errorf("flush staged source: %w", syncErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close staged source: %w", closeErr)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func ensureDirectChild(parent, child string) error {
	parentAbs, err := paths.Canonical(parent)
	if err != nil {
		return fmt.Errorf("resolve inbox: %w", err)
	}
	childAbs, err := paths.Canonical(child)
	if err != nil {
		return fmt.Errorf("resolve inbox item: %w", err)
	}
	parentDir := filepath.Clean(parentAbs)
	childDir := filepath.Clean(filepath.Dir(childAbs))
	if !paths.Same(parentDir, childDir) {
		return errors.New("import source must be a direct child of a configured Inbox")
	}
	return nil
}
