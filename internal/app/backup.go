package app

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	backupFormat                = "tavern-shelf-backup"
	backupVersion               = 1
	MaxBackupSize         int64 = 64 << 30
	maxBackupItemSize           = 4 << 30
	maxBackupItems              = 100_000
	maxBackupManifestSize       = 8 << 20
)

var errBackupItemTooLarge = errors.New("backup item exceeds the 4 GiB limit")

type BackupItem struct {
	Kind           string    `json:"kind"`
	Name           string    `json:"name"`
	SourceFilename string    `json:"sourceFilename"`
	SourceHash     string    `json:"sourceHash"`
	ImportedAt     time.Time `json:"importedAt"`
	ArchivePath    string    `json:"archivePath"`
}

type BackupManifest struct {
	Format    string       `json:"format"`
	Version   int          `json:"version"`
	CreatedAt time.Time    `json:"createdAt"`
	Items     []BackupItem `json:"items"`
}

type BackupSummary struct {
	Items int `json:"items"`
}

type RestoreIssue struct {
	File  string `json:"file"`
	Error string `json:"error"`
}

type RestoreSummary struct {
	Total      int            `json:"total"`
	Imported   int            `json:"imported"`
	Duplicates int            `json:"duplicates"`
	Failed     int            `json:"failed"`
	Issues     []RestoreIssue `json:"issues,omitempty"`
}

func (a *App) WriteBackup(ctx context.Context, destination io.Writer) (BackupSummary, error) {
	characters, err := a.Store.List(ctx)
	if err != nil {
		return BackupSummary{}, fmt.Errorf("list characters for backup: %w", err)
	}
	resources, err := a.Store.ListResources(ctx, "")
	if err != nil {
		return BackupSummary{}, fmt.Errorf("list resources for backup: %w", err)
	}
	archive := zip.NewWriter(destination)
	manifest := BackupManifest{Format: backupFormat, Version: backupVersion, CreatedAt: time.Now().UTC()}
	for _, character := range characters {
		path := filepath.Join(a.Paths.Library, character.SourceRelPath)
		if !isWithin(a.Paths.Library, path) {
			_ = archive.Close()
			return BackupSummary{}, errors.New("refusing to back up an unexpected character source path")
		}
		item := BackupItem{
			Kind: "character", Name: character.Name, SourceFilename: character.SourceFilename,
			SourceHash: character.SourceHash, ImportedAt: character.ImportedAt,
			ArchivePath: backupSourcePath("character", character.SourceHash, path),
		}
		if err := addBackupSource(archive, item.ArchivePath, path, item.SourceHash); err != nil {
			_ = archive.Close()
			return BackupSummary{}, err
		}
		manifest.Items = append(manifest.Items, item)
	}
	for _, resource := range resources {
		path := filepath.Join(a.Paths.Library, resource.SourceRelPath)
		if !isWithin(a.Paths.Library, path) {
			_ = archive.Close()
			return BackupSummary{}, errors.New("refusing to back up an unexpected resource source path")
		}
		item := BackupItem{
			Kind: resource.Kind, Name: resource.Name, SourceFilename: resource.SourceFilename,
			SourceHash: resource.SourceHash, ImportedAt: resource.ImportedAt,
			ArchivePath: backupSourcePath(resource.Kind, resource.SourceHash, path),
		}
		if err := addBackupSource(archive, item.ArchivePath, path, item.SourceHash); err != nil {
			_ = archive.Close()
			return BackupSummary{}, err
		}
		manifest.Items = append(manifest.Items, item)
	}
	header := &zip.FileHeader{Name: "backup.json", Method: zip.Deflate}
	header.SetModTime(manifest.CreatedAt)
	entry, err := archive.CreateHeader(header)
	if err != nil {
		_ = archive.Close()
		return BackupSummary{}, fmt.Errorf("create backup manifest: %w", err)
	}
	encoder := json.NewEncoder(entry)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		_ = archive.Close()
		return BackupSummary{}, fmt.Errorf("write backup manifest: %w", err)
	}
	if err := archive.Close(); err != nil {
		return BackupSummary{}, fmt.Errorf("finish backup archive: %w", err)
	}
	return BackupSummary{Items: len(manifest.Items)}, nil
}

func (a *App) RestoreBackup(ctx context.Context, source io.Reader) (RestoreSummary, error) {
	temp, err := os.CreateTemp(a.Paths.Staging, "restore-*.zip")
	if err != nil {
		return RestoreSummary{}, fmt.Errorf("create restore staging file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	written, copyErr := io.Copy(temp, io.LimitReader(source, MaxBackupSize+1))
	closeErr := temp.Close()
	if written > MaxBackupSize {
		return RestoreSummary{}, errors.New("backup exceeds the 64 GiB limit")
	}
	if copyErr != nil {
		return RestoreSummary{}, fmt.Errorf("stage backup: %w", copyErr)
	}
	if closeErr != nil {
		return RestoreSummary{}, fmt.Errorf("close staged backup: %w", closeErr)
	}
	reader, err := zip.OpenReader(tempPath)
	if err != nil {
		return RestoreSummary{}, fmt.Errorf("open backup archive: %w", err)
	}
	defer reader.Close()
	manifest, sources, err := readBackupManifest(reader.File)
	if err != nil {
		return RestoreSummary{}, err
	}
	if err := validateBackupSources(manifest, sources); err != nil {
		return RestoreSummary{}, err
	}
	summary := RestoreSummary{Total: len(manifest.Items)}
	for _, item := range manifest.Items {
		entry := sources[item.ArchivePath]
		stream, err := entry.Open()
		if err != nil {
			summary.addIssue(item.SourceFilename, err)
			continue
		}
		result, importErr := a.importReader(ctx, item.SourceFilename, stream, maxBackupItemSize, errBackupItemTooLarge, "restore-item-")
		closeErr := stream.Close()
		if importErr == nil && closeErr != nil {
			importErr = closeErr
		}
		if importErr != nil {
			summary.addIssue(item.SourceFilename, importErr)
			continue
		}
		id := result.Character.ID
		if result.Resource.ID != "" {
			id = result.Resource.ID
		}
		if !strings.EqualFold(id, item.SourceHash) {
			summary.addIssue(item.SourceFilename, errors.New("restored content hash does not match backup manifest"))
			continue
		}
		if result.Duplicate {
			summary.Duplicates++
			continue
		}
		summary.Imported++
		if !item.ImportedAt.IsZero() {
			_ = a.Store.SetImportedAt(ctx, result.Kind, id, item.ImportedAt)
		}
	}
	return summary, nil
}

func (s *RestoreSummary) addIssue(file string, err error) {
	s.Failed++
	s.Issues = append(s.Issues, RestoreIssue{File: file, Error: err.Error()})
}

func backupSourcePath(kind, hash, path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	return "sources/" + kind + "/" + hash + ext
}

func addBackupSource(archive *zip.Writer, archivePath, sourcePath, expectedHash string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open managed source for backup: %w", err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("inspect managed source for backup: %w", err)
	}
	header := &zip.FileHeader{Name: archivePath, Method: zip.Deflate}
	header.SetModTime(info.ModTime())
	entry, err := archive.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create backup source entry: %w", err)
	}
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(entry, hash), source); err != nil {
		return fmt.Errorf("write backup source: %w", err)
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expectedHash) {
		return errors.New("managed source changed or failed its SHA-256 check during backup")
	}
	return nil
}

func readBackupManifest(files []*zip.File) (BackupManifest, map[string]*zip.File, error) {
	var manifestFile *zip.File
	sources := make(map[string]*zip.File)
	for _, file := range files {
		if file.Name == "backup.json" {
			manifestFile = file
			continue
		}
		sources[file.Name] = file
	}
	if manifestFile == nil || manifestFile.UncompressedSize64 > maxBackupManifestSize {
		return BackupManifest{}, nil, errors.New("backup manifest is missing or too large")
	}
	stream, err := manifestFile.Open()
	if err != nil {
		return BackupManifest{}, nil, fmt.Errorf("open backup manifest: %w", err)
	}
	defer stream.Close()
	var manifest BackupManifest
	decoder := json.NewDecoder(io.LimitReader(stream, maxBackupManifestSize+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return BackupManifest{}, nil, fmt.Errorf("decode backup manifest: %w", err)
	}
	if manifest.Format != backupFormat || manifest.Version != backupVersion {
		return BackupManifest{}, nil, errors.New("unsupported Tavern Shelf backup format or version")
	}
	if len(manifest.Items) > maxBackupItems {
		return BackupManifest{}, nil, errors.New("backup contains too many items")
	}
	return manifest, sources, nil
}

func validateBackupSources(manifest BackupManifest, sources map[string]*zip.File) error {
	seen := make(map[string]struct{}, len(manifest.Items))
	var expandedSize uint64
	for _, item := range manifest.Items {
		if item.Kind != "character" && item.Kind != "worldbook" && item.Kind != "preset" {
			return fmt.Errorf("backup contains invalid item kind %q", item.Kind)
		}
		if len(item.SourceHash) != sha256.Size*2 {
			return fmt.Errorf("backup item %q has an invalid source hash", item.SourceFilename)
		}
		if _, err := hex.DecodeString(item.SourceHash); err != nil {
			return fmt.Errorf("backup item %q has an invalid source hash", item.SourceFilename)
		}
		if item.SourceFilename != filepath.Base(strings.ReplaceAll(item.SourceFilename, "\\", "/")) {
			return fmt.Errorf("backup item has an unsafe source filename %q", item.SourceFilename)
		}
		if _, exists := seen[item.ArchivePath]; exists {
			return fmt.Errorf("backup contains duplicate source entry %q", item.ArchivePath)
		}
		seen[item.ArchivePath] = struct{}{}
		entry, ok := sources[item.ArchivePath]
		if !ok || entry.FileInfo().IsDir() || entry.UncompressedSize64 > uint64(maxBackupItemSize) {
			return fmt.Errorf("backup source %q is missing or too large", item.ArchivePath)
		}
		if entry.UncompressedSize64 > uint64(MaxBackupSize)-expandedSize {
			return errors.New("expanded backup exceeds the 64 GiB limit")
		}
		expandedSize += entry.UncompressedSize64
		stream, err := entry.Open()
		if err != nil {
			return fmt.Errorf("open backup source %q: %w", item.ArchivePath, err)
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, io.LimitReader(stream, maxBackupItemSize+1))
		closeErr := stream.Close()
		if copyErr != nil || closeErr != nil {
			return fmt.Errorf("read backup source %q", item.ArchivePath)
		}
		if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), item.SourceHash) {
			return fmt.Errorf("backup source %q failed its SHA-256 check", item.ArchivePath)
		}
	}
	return nil
}
