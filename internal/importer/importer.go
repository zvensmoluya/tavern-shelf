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
	"github.com/openai/tavern-shelf/internal/store"
)

type Result struct {
	Character library.Character
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
	if err := ensureDirectChild(i.paths.Inbox, source); err != nil {
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
	metadata, err := card.ParseFile(source)
	if err != nil {
		return Result{}, err
	}
	hash, err := hashFile(source)
	if err != nil {
		return Result{}, err
	}
	if existing, err := i.store.GetByHash(ctx, hash); err == nil {
		if err := i.archiveDuplicate(source, hash); err != nil {
			return Result{}, err
		}
		return Result{Character: existing, Duplicate: true}, nil
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

	character := library.Character{
		ID:             hash,
		SourceHash:     hash,
		Name:           metadata.Name,
		Creator:        metadata.Creator,
		Spec:           metadata.Spec,
		SpecVersion:    metadata.SpecVersion,
		Tags:           metadata.Tags,
		HasWorldbook:   metadata.HasWorldbook,
		HasRegex:       metadata.HasRegex,
		HasExtensions:  metadata.HasExtensions,
		HasInteractive: metadata.HasInteractive,
		SourceFormat:   metadata.SourceFormat,
		SourceIsImage:  metadata.SourceIsImage,
		SourceFilename: filepath.Base(source),
		SourceRelPath:  relSource,
		SourceSize:     info.Size(),
		ImportedAt:     i.now().UTC(),
		Manifest:       metadata.Manifest,
	}
	if err := i.store.Create(ctx, character); err != nil {
		return Result{}, err
	}
	committed = true
	if err := os.Remove(source); err != nil {
		// The managed copy and database row are complete. A later scan will identify
		// this leftover as an exact duplicate and archive it safely.
		return Result{Character: character}, fmt.Errorf("remove imported inbox file: %w", err)
	}
	if _, err := os.Stat(finalSource); err != nil {
		return Result{Character: character}, fmt.Errorf("verify managed source: %w", err)
	}
	return Result{Character: character}, nil
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
	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		return fmt.Errorf("resolve inbox: %w", err)
	}
	childAbs, err := filepath.Abs(child)
	if err != nil {
		return fmt.Errorf("resolve inbox item: %w", err)
	}
	if filepath.Dir(childAbs) != parentAbs {
		return errors.New("import source must be a direct child of the Inbox")
	}
	return nil
}
