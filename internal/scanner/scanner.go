package scanner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/openai/tavern-shelf/internal/card"
	"github.com/openai/tavern-shelf/internal/importer"
)

type Config struct {
	Inbox        string
	Inboxes      []string
	ManagedInbox string
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
	imported    bool
}

type Scanner struct {
	inboxes      []string
	managedInbox string
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
	inboxes := append([]string(nil), config.Inboxes...)
	if len(inboxes) == 0 && config.Inbox != "" {
		inboxes = []string{config.Inbox}
	}
	managedInbox := config.ManagedInbox
	if managedInbox == "" && config.Inbox != "" {
		managedInbox = config.Inbox
	}
	return &Scanner{
		inboxes: inboxes, managedInbox: managedInbox, interval: config.Interval, stableFor: config.StableFor,
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
	seen := make(map[string]struct{})
	pending := 0
	var scanErrors []error
	for _, inbox := range s.Inboxes() {
		count, err := s.scanInbox(ctx, now, inbox, seen)
		pending += count
		if err != nil {
			scanErrors = append(scanErrors, err)
			s.recordError(now, inbox, err)
		}
	}
	s.pruneObservations(seen)
	s.mu.Lock()
	s.status.Pending = pending
	s.status.LastScanAt = now.UTC()
	s.mu.Unlock()
	return errors.Join(scanErrors...)
}

func (s *Scanner) scanInbox(ctx context.Context, now time.Time, inbox string, seen map[string]struct{}) (int, error) {
	entries, err := os.ReadDir(inbox)
	if err != nil {
		return 0, fmt.Errorf("read Inbox %q: %w", inbox, err)
	}
	sort.Slice(entries, func(a, b int) bool { return entries[a].Name() < entries[b].Name() })
	pending := 0
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(inbox, entry.Name())
		if !card.Supported(path) {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		seen[path] = struct{}{}
		obs, ok := s.getObservation(path)
		fingerprintChanged := !ok || obs.size != info.Size() || obs.modified != info.ModTime().UnixNano()
		if fingerprintChanged {
			s.setObservation(path, observation{size: info.Size(), modified: info.ModTime().UnixNano(), stableSince: now})
			pending++
			continue
		}
		if obs.imported {
			continue
		}
		pending++
		if now.Before(obs.nextRetry) || now.Sub(obs.stableSince) < s.stableFor {
			continue
		}
		moveSource := samePath(inbox, s.managedInbox)
		var result importer.Result
		if moveSource {
			result, err = s.importer.Import(ctx, path)
		} else {
			result, err = s.importer.ImportFrom(ctx, inbox, path)
		}
		if err != nil {
			obs.nextRetry = now.Add(s.retryAfter)
			s.setObservation(path, obs)
			s.recordError(now, entry.Name(), err)
			s.logger.Warn("character import failed", "file", entry.Name(), "error", err, "retry_at", obs.nextRetry)
			continue
		}
		if moveSource {
			s.deleteObservation(path)
		} else {
			obs.imported = true
			s.setObservation(path, obs)
		}
		pending--
		s.recordImport(now)
		if result.Duplicate {
			s.logger.Info("archived duplicate Shelf resource", "file", entry.Name(), "kind", result.Kind, "name", result.Name)
		} else {
			s.logger.Info("imported Shelf resource", "file", entry.Name(), "kind", result.Kind, "name", result.Name)
		}
		if s.onLibraryHit != nil {
			s.onLibraryHit()
		}
	}
	return pending, nil
}

func samePath(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func (s *Scanner) Inboxes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.inboxes...)
}

func (s *Scanner) SetInboxes(inboxes []string) {
	s.mu.Lock()
	s.inboxes = append([]string(nil), inboxes...)
	s.mu.Unlock()
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
