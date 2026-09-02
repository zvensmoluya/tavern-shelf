package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/zvensmoluya/tavern-shelf/internal/adaptation"
	"github.com/zvensmoluya/tavern-shelf/internal/library"
	"github.com/zvensmoluya/tavern-shelf/internal/store"
)

const (
	programViewFilename = "program-view-v1.json"
	artifactFilename    = "adaptation-v1.json"
)

func (a *App) BuildProgramView(ctx context.Context, characterID string) (adaptation.ProgramView, library.Adaptation, error) {
	character, sourcePath, err := a.SourcePath(ctx, characterID)
	if err != nil {
		return adaptation.ProgramView{}, library.Adaptation{}, err
	}
	actualSourceHash, err := hashFile(sourcePath)
	if err != nil {
		return adaptation.ProgramView{}, library.Adaptation{}, fmt.Errorf("hash managed character source: %w", err)
	}
	if actualSourceHash != character.SourceHash {
		return adaptation.ProgramView{}, library.Adaptation{}, errors.New("managed character source no longer matches its immutable source hash")
	}
	view, err := adaptation.ExtractProgramView(sourcePath, character.SourceHash)
	if err != nil {
		return adaptation.ProgramView{}, library.Adaptation{}, err
	}
	raw, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		return adaptation.ProgramView{}, library.Adaptation{}, fmt.Errorf("encode Program View: %w", err)
	}
	raw = append(raw, '\n')
	derivedDir := filepath.Join(filepath.Dir(sourcePath), "derived")
	path := filepath.Join(derivedDir, programViewFilename)
	if err := writeDerivedFile(path, raw); err != nil {
		return adaptation.ProgramView{}, library.Adaptation{}, err
	}
	rel, err := filepath.Rel(a.Paths.Library, path)
	if err != nil || !isWithin(a.Paths.Library, path) {
		return adaptation.ProgramView{}, library.Adaptation{}, errors.New("derived Program View path escapes the managed Library")
	}
	record := library.Adaptation{
		CharacterID: character.ID, SourceHash: character.SourceHash,
		ProgramViewPath: rel, ProgramViewHash: hashBytes(raw),
		Status: "PROGRAM_VIEW_READY", UpdatedAt: time.Now().UTC(),
	}
	if previous, getErr := a.Store.GetAdaptation(ctx, character.ID); getErr == nil && previous.SourceHash == character.SourceHash {
		record.ArtifactPath = previous.ArtifactPath
		record.ArtifactHash = previous.ArtifactHash
		record.ArtifactSize = previous.ArtifactSize
		record.CompilerID = previous.CompilerID
		record.CompilerVersion = previous.CompilerVersion
		record.CompilerModel = previous.CompilerModel
		if previous.ArtifactPath != "" {
			record.Status = previous.Status
		}
	} else if getErr != nil && !errors.Is(getErr, store.ErrNotFound) {
		return adaptation.ProgramView{}, library.Adaptation{}, getErr
	}
	if err := a.Store.UpsertAdaptation(ctx, record); err != nil {
		return adaptation.ProgramView{}, library.Adaptation{}, err
	}
	return view, record, nil
}

func (a *App) InstallAdaptation(ctx context.Context, characterID string, source io.Reader) (library.Adaptation, error) {
	character, sourcePath, err := a.SourcePath(ctx, characterID)
	if err != nil {
		return library.Adaptation{}, err
	}
	artifact, raw, err := adaptation.DecodeArtifact(source)
	if err != nil {
		return library.Adaptation{}, err
	}
	view, record, err := a.BuildProgramView(ctx, character.ID)
	if err != nil {
		return library.Adaptation{}, err
	}
	if issues := adaptation.ValidateArtifactForProgramView(artifact, view); len(issues) != 0 {
		return library.Adaptation{}, fmt.Errorf("adaptation artifact failed validation: %s at %s", issues[0].Code, issues[0].Path)
	}
	path := filepath.Join(filepath.Dir(sourcePath), "derived", artifactFilename)
	if !isWithin(a.Paths.Library, path) {
		return library.Adaptation{}, errors.New("derived adaptation path escapes the managed Library")
	}
	if err := writeDerivedFile(path, raw); err != nil {
		return library.Adaptation{}, err
	}
	rel, err := filepath.Rel(a.Paths.Library, path)
	if err != nil {
		return library.Adaptation{}, fmt.Errorf("resolve adaptation path: %w", err)
	}
	record.ArtifactPath = rel
	record.ArtifactHash = hashBytes(raw)
	record.ArtifactSize = int64(len(raw))
	record.CompilerID = artifact.Compiler.ID
	record.CompilerVersion = artifact.Compiler.Version
	if artifact.Compiler.Model != nil {
		record.CompilerModel = *artifact.Compiler.Model
	}
	record.Status = artifact.Status
	record.UpdatedAt = time.Now().UTC()
	if err := a.Store.UpsertAdaptation(ctx, record); err != nil {
		return library.Adaptation{}, err
	}
	return record, nil
}

func (a *App) AdaptationPath(ctx context.Context, characterID string) (library.Adaptation, string, error) {
	record, err := a.Store.GetAdaptation(ctx, characterID)
	if err != nil {
		return library.Adaptation{}, "", err
	}
	if record.ArtifactPath == "" {
		return library.Adaptation{}, "", store.ErrNotFound
	}
	path := filepath.Join(a.Paths.Library, record.ArtifactPath)
	if !isWithin(a.Paths.Library, path) {
		return library.Adaptation{}, "", errors.New("stored adaptation path escapes the managed Library")
	}
	return record, path, nil
}

func writeDerivedFile(path string, raw []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create derived content directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".adaptation-*")
	if err != nil {
		return fmt.Errorf("stage derived content: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set derived content permissions: %w", err)
	}
	if _, err := io.Copy(temporary, bytes.NewReader(raw)); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write derived content: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync derived content: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close derived content: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("replace derived content: %w", err)
		}
		if retryErr := os.Rename(temporaryPath, path); retryErr != nil {
			return fmt.Errorf("replace derived content: %w", retryErr)
		}
	}
	return nil
}

func hashFile(path string) (string, error) {
	stream, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer stream.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, stream); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func hashBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
