package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zvensmoluya/tavern-shelf/internal/card"
	"github.com/zvensmoluya/tavern-shelf/internal/importer"
)

const MaxUploadSize int64 = 64 << 20

var ErrUploadTooLarge = errors.New("uploaded file exceeds the 64 MiB limit")

// ImportUpload copies an uploaded resource into a private staging directory
// before importing it. The user's original dragged file is never modified.
func (a *App) ImportUpload(ctx context.Context, filename string, source io.Reader) (importer.Result, error) {
	return a.importReader(ctx, filename, source, MaxUploadSize, ErrUploadTooLarge, "upload-", false)
}

// ImportCharacterUpload is the connector-facing variant that rejects standalone
// JSON resources before they can be committed to the Library.
func (a *App) ImportCharacterUpload(ctx context.Context, filename string, source io.Reader) (importer.Result, error) {
	return a.importReader(ctx, filename, source, MaxUploadSize, ErrUploadTooLarge, "connector-", true)
}

func (a *App) importReader(ctx context.Context, filename string, source io.Reader, limit int64, tooLarge error, prefix string, characterOnly bool) (importer.Result, error) {
	filename = strings.TrimSpace(strings.ReplaceAll(filename, "\\", "/"))
	filename = filepath.Base(filename)
	if filename == "" || filename == "." || !card.Supported(filename) {
		return importer.Result{}, errors.New("only PNG character cards and JSON Shelf resources are supported")
	}
	tempDir, err := os.MkdirTemp(a.Paths.Staging, prefix)
	if err != nil {
		return importer.Result{}, fmt.Errorf("create upload staging directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	tempSource := filepath.Join(tempDir, filename)
	file, err := os.OpenFile(tempSource, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return importer.Result{}, fmt.Errorf("create staged upload: %w", err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(source, limit+1))
	syncErr := file.Sync()
	closeErr := file.Close()
	if written > limit {
		return importer.Result{}, tooLarge
	}
	if copyErr != nil {
		return importer.Result{}, fmt.Errorf("copy upload to staging: %w", copyErr)
	}
	if syncErr != nil {
		return importer.Result{}, fmt.Errorf("flush staged upload: %w", syncErr)
	}
	if closeErr != nil {
		return importer.Result{}, fmt.Errorf("close staged upload: %w", closeErr)
	}
	if characterOnly {
		if _, err := card.ParseFile(tempSource); err != nil {
			return importer.Result{}, fmt.Errorf("connector upload must be a character card: %w", err)
		}
	}
	result, err := a.Importer.ImportFrom(ctx, tempDir, tempSource)
	if err != nil {
		return importer.Result{}, err
	}
	if a.onLibraryHit != nil {
		a.onLibraryHit()
	}
	return result, nil
}
