package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/openai/tavern-shelf/internal/card"
	"github.com/openai/tavern-shelf/internal/importer"
)

type Config struct {
	Inbox        string
	Interval     time.Duration
	StableFor    time.Duration
	RetryAfter   time.Duration
	Import       *importer.Importer
	Logger       *slog.Logger
	OnLibraryHit func()
}

type Status struct {
	Running       bool      `json:"running"`
	Pending       int       `json:"pending"`
	LastScanAt    time.Time `json:"lastScanAt,omitempty"`
	LastImportAt  time.Time `json:"lastImportAt,omitempty"`
	LastErrorAt   time.Time `json:"lastErrorAt,omitempty"`
	LastErrorFile string    `json:"lastErrorFile,omitempty"`
	LastError     string    `json:"lastError,omitempty"`
}

type observation struct {
	size        int64
	modified    int64
	stableSince time.Time
	nextRetry   time.Time
}

type Scanner struct {
	inbox        string
	interval     time.Duration
	stableFor    time.Duration
	retryAfter   time.Duration
	importer     *importer.Importer
	logger       *slog.Logger
	onLibraryHit func()

	mu           sync.RWMutex
	observations map[string]observation
	status       Status
}

func New(config Config) *Scanner {
	if config.Interval <= 0 {
		config.Interval = 2 * time.Second
	}
	if config.StableFor <= 0 {
		config.StableFor = 2 * time.Second
	}
	if config.RetryAfter <= 0 {
		config.RetryAfter = 5 * time.Minute
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	return &Scanner{
		inbox: config.Inbox, interval: config.Interval, stableFor: config.StableFor,
		retryAfter: config.RetryAfter, importer: config.Import, logger: config.Logger,
		onLibraryHit: config.OnLibraryHit, observations: make(map[string]observation),
	}
}

func (s *Scanner) Run(ctx context.Context) error {
	s.setRunning(true)
	defer s.setRunning(false)
	if err := s.ScanOnce(ctx, time.Now()); err != nil {
		s.logger.Error("inbox scan failed", "error", err)
	}
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			if err := s.ScanOnce(ctx, now); err != nil {
				s.logger.Error("inbox scan failed", "error", err)
			}
		}
	}
}

func (s *Scanner) ScanOnce(ctx context.Context, now time.Time) error {
	entries, err := os.ReadDir(s.inbox)
	if err != nil {
		return fmt.Errorf("read Inbox: %w", err)
	}
	sort.Slice(entries, func(a, b int) bool { return entries[a].Name() < entries[b].Name() })
	seen := make(map[string]struct{}, len(entries))
	pending := 0
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(s.inbox, entry.Name())
		if !card.Supported(path) {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		seen[path] = struct{}{}
		pending++
		obs, ok := s.getObservation(path)
		fingerprintChanged := !ok || obs.size != info.Size() || obs.modified != info.ModTime().UnixNano()
		if fingerprintChanged {
			s.setObservation(path, observation{size: info.Size(), modified: info.ModTime().UnixNano(), stableSince: now})
			continue
		}
		if now.Before(obs.nextRetry) || now.Sub(obs.stableSince) < s.stableFor {
			continue
		}
		result, err := s.importer.Import(ctx, path)
		if err != nil {
			obs.nextRetry = now.Add(s.retryAfter)
			s.setObservation(path, obs)
			s.recordError(now, entry.Name(), err)
			s.logger.Warn("character import failed", "file", entry.Name(), "error", err, "retry_at", obs.nextRetry)
			continue
		}
		s.deleteObservation(path)
		pending--
		s.recordImport(now)
		if result.Duplicate {
			s.logger.Info("archived duplicate character card", "file", entry.Name(), "character", result.Character.Name)
		} else {
			s.logger.Info("imported character card", "file", entry.Name(), "character", result.Character.Name)
		}
		if s.onLibraryHit != nil {
			s.onLibraryHit()
		}
	}
	s.pruneObservations(seen)
	s.mu.Lock()
	s.status.Pending = pending
	s.status.LastScanAt = now.UTC()
	s.mu.Unlock()
	return nil
}

func (s *Scanner) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *Scanner) setRunning(value bool) {
	s.mu.Lock()
	s.status.Running = value
	s.mu.Unlock()
}

func (s *Scanner) recordImport(now time.Time) {
	s.mu.Lock()
	s.status.LastImportAt = now.UTC()
	s.status.LastError = ""
	s.status.LastErrorFile = ""
	s.mu.Unlock()
}

func (s *Scanner) recordError(now time.Time, name string, err error) {
	s.mu.Lock()
	s.status.LastErrorAt = now.UTC()
	s.status.LastErrorFile = name
	s.status.LastError = err.Error()
	s.mu.Unlock()
}

func (s *Scanner) getObservation(path string) (observation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	obs, ok := s.observations[path]
	return obs, ok
}

func (s *Scanner) setObservation(path string, obs observation) {
	s.mu.Lock()
	s.observations[path] = obs
	s.mu.Unlock()
}

func (s *Scanner) deleteObservation(path string) {
	s.mu.Lock()
	delete(s.observations, path)
	s.mu.Unlock()
}

func (s *Scanner) pruneObservations(seen map[string]struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for path := range s.observations {
		if _, ok := seen[path]; !ok {
			delete(s.observations, path)
		}
	}
}
