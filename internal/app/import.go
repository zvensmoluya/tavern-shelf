package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/openai/tavern-shelf/internal/card"
	"github.com/openai/tavern-shelf/internal/importer"
)

const MaxUploadSize int64 = 64 << 20

var ErrUploadTooLarge = errors.New("uploaded file exceeds the 64 MiB limit")

// ImportUpload copies an uploaded resource into a private staging directory
// before importing it. The user's original dragged file is never modified.
func (a *App) ImportUpload(ctx context.Context, filename string, source io.Reader) (importer.Result, error) {
	filename = strings.TrimSpace(strings.ReplaceAll(filename, "\\", "/"))
	filename = filepath.Base(filename)
	if filename == "" || filename == "." || !card.Supported(filename) {
		return importer.Result{}, errors.New("only PNG character cards and JSON Shelf resources are supported")
	}
	tempDir, err := os.MkdirTemp(a.Paths.Staging, "upload-")
	if err != nil {
		return importer.Result{}, fmt.Errorf("create upload staging directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	tempSource := filepath.Join(tempDir, filename)
	file, err := os.OpenFile(tempSource, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return importer.Result{}, fmt.Errorf("create staged upload: %w", err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(source, MaxUploadSize+1))
	syncErr := file.Sync()
	closeErr := file.Close()
	if written > MaxUploadSize {
		return importer.Result{}, ErrUploadTooLarge
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
	result, err := a.Importer.ImportFrom(ctx, tempDir, tempSource)
	if err != nil {
		return importer.Result{}, err
	}
	if a.onLibraryHit != nil {
		a.onLibraryHit()
	}
	return result, nil
}
