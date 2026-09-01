package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/openai/tavern-shelf/internal/card"
)

type OneShotScanIssue struct {
	File  string `json:"file"`
	Error string `json:"error"`
}

type OneShotScanStatus struct {
	ID          string             `json:"id,omitempty"`
	Directory   string             `json:"directory,omitempty"`
	Running     bool               `json:"running"`
	Total       int                `json:"total"`
	Imported    int                `json:"imported"`
	Duplicates  int                `json:"duplicates"`
	Failed      int                `json:"failed"`
	StartedAt   time.Time          `json:"startedAt,omitempty"`
	CompletedAt time.Time          `json:"completedAt,omitempty"`
	Issues      []OneShotScanIssue `json:"issues,omitempty"`
}

type scanCandidate struct {
	name     string
	path     string
	size     int64
	modified int64
}

func (a *App) StartScanOnce(ctx context.Context, directory string) (OneShotScanStatus, error) {
	if err := ctx.Err(); err != nil {
		return OneShotScanStatus{}, err
	}
	directory, err := a.validateInbox(directory)
	if err != nil {
		return OneShotScanStatus{}, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return OneShotScanStatus{}, fmt.Errorf("read one-time scan directory: %w", err)
	}
	candidates := make([]scanCandidate, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || !card.Supported(entry.Name()) {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() {
			continue
		}
		candidates = append(candidates, scanCandidate{
			name: entry.Name(), path: filepath.Join(directory, entry.Name()),
			size: info.Size(), modified: info.ModTime().UnixNano(),
		})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].name < candidates[j].name })

	a.scanMu.Lock()
	if a.oneShotScan.Running {
		a.scanMu.Unlock()
		return OneShotScanStatus{}, errors.New("a one-time scan is already running")
	}
	status := OneShotScanStatus{
		ID: fmt.Sprintf("scan-%d", time.Now().UnixNano()), Directory: directory,
		Running: true, Total: len(candidates), StartedAt: time.Now().UTC(),
	}
	a.oneShotScan = status
	a.scanMu.Unlock()

	a.workWG.Add(1)
	go a.runOneShotScan(directory, candidates)
	return status, nil
}

func (a *App) OneShotScanStatus() OneShotScanStatus {
	a.scanMu.RLock()
	defer a.scanMu.RUnlock()
	status := a.oneShotScan
	status.Issues = append([]OneShotScanIssue(nil), status.Issues...)
	return status
}

func (a *App) runOneShotScan(directory string, candidates []scanCandidate) {
	defer a.workWG.Done()
	if !waitFor(a.runCtx, a.stableFor) {
		a.finishOneShotScan()
		return
	}
	for _, candidate := range candidates {
		select {
		case <-a.runCtx.Done():
			a.finishOneShotScan()
			return
		default:
		}
		info, err := os.Stat(candidate.path)
		if err != nil || !info.Mode().IsRegular() || info.Size() != candidate.size || info.ModTime().UnixNano() != candidate.modified {
			if err == nil {
				err = errors.New("file changed while waiting for it to become stable")
			}
			a.recordOneShotIssue(candidate.name, err)
			continue
		}
		result, err := a.Importer.ImportFrom(a.runCtx, directory, candidate.path)
		if err != nil {
			a.recordOneShotIssue(candidate.name, err)
			continue
		}
		a.scanMu.Lock()
		if result.Duplicate {
			a.oneShotScan.Duplicates++
		} else {
			a.oneShotScan.Imported++
		}
		a.scanMu.Unlock()
		if a.onLibraryHit != nil {
			a.onLibraryHit()
		}
	}
	a.finishOneShotScan()
}

func (a *App) recordOneShotIssue(filename string, err error) {
	if err == nil {
		err = errors.New("file is no longer available")
	}
	a.scanMu.Lock()
	a.oneShotScan.Failed++
	a.oneShotScan.Issues = append(a.oneShotScan.Issues, OneShotScanIssue{File: filename, Error: err.Error()})
	a.scanMu.Unlock()
}

func (a *App) finishOneShotScan() {
	a.scanMu.Lock()
	a.oneShotScan.Running = false
	a.oneShotScan.CompletedAt = time.Now().UTC()
	a.scanMu.Unlock()
}

func waitFor(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
