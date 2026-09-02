package connector

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	Protocol        = "tavern-shelf-connector"
	Version         = 1
	pairingLifetime = 5 * time.Minute
	maxPairFailures = 5
)

var (
	ErrInvalidCode  = errors.New("pairing code is invalid or expired")
	ErrUnauthorized = errors.New("connector authorization is invalid")
)

type Client struct {
	Name     string    `json:"name"`
	Version  string    `json:"version,omitempty"`
	PairedAt time.Time `json:"pairedAt"`
}

type Status struct {
	Protocol         string    `json:"protocol"`
	Version          int       `json:"version"`
	Listening        bool      `json:"listening"`
	Address          string    `json:"address,omitempty"`
	ListenerError    string    `json:"listenerError,omitempty"`
	Paired           bool      `json:"paired"`
	Client           *Client   `json:"client,omitempty"`
	PairingExpiresAt time.Time `json:"pairingExpiresAt,omitempty"`
}

type Pairing struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type storedState struct {
	TokenHash string  `json:"tokenHash"`
	Client    *Client `json:"client,omitempty"`
}

type Service struct {
	mu            sync.RWMutex
	path          string
	now           func() time.Time
	state         storedState
	pairCode      string
	pairExpiresAt time.Time
	pairFailures  int
	listening     bool
	address       string
	listenerError string
}

func Open(path string) (*Service, error) {
	s := &Service{path: path, now: time.Now}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read connector state: %w", err)
	}
	if err := json.Unmarshal(raw, &s.state); err != nil {
		return nil, fmt.Errorf("decode connector state: %w", err)
	}
	return s, nil
}

func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := Status{Protocol: Protocol, Version: Version, Listening: s.listening, Address: s.address, ListenerError: s.listenerError, Paired: s.state.TokenHash != "", Client: s.state.Client}
	if s.pairCode != "" && s.now().Before(s.pairExpiresAt) {
		status.PairingExpiresAt = s.pairExpiresAt
	}
	return status
}

func (s *Service) SetListener(address string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.address = address
	s.listening = err == nil
	s.listenerError = ""
	if err != nil {
		s.listenerError = err.Error()
	}
}

func (s *Service) BeginPairing() (Pairing, error) {
	limit := big.NewInt(1_000_000)
	value, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return Pairing{}, fmt.Errorf("create pairing code: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pairCode = fmt.Sprintf("%06d", value.Int64())
	s.pairExpiresAt = s.now().Add(pairingLifetime).UTC()
	s.pairFailures = 0
	return Pairing{Code: s.pairCode, ExpiresAt: s.pairExpiresAt}, nil
}

func (s *Service) Pair(code, name, version string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	valid := s.pairCode != "" && now.Before(s.pairExpiresAt) && subtle.ConstantTimeCompare([]byte(code), []byte(s.pairCode)) == 1
	if !valid {
		s.pairFailures++
		if s.pairFailures >= maxPairFailures || !now.Before(s.pairExpiresAt) {
			s.clearPairingLocked()
		}
		return "", ErrInvalidCode
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("create connector token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	if name == "" {
		name = "SillyTavern"
	}
	next := storedState{TokenHash: hex.EncodeToString(hash[:]), Client: &Client{Name: name, Version: version, PairedAt: now}}
	if err := s.persistLocked(next); err != nil {
		return "", err
	}
	s.state = next
	s.clearPairingLocked()
	return token, nil
}

func (s *Service) Authorize(token string) bool {
	s.mu.RLock()
	stored := s.state.TokenHash
	s.mu.RUnlock()
	if token == "" || stored == "" {
		return false
	}
	hash := sha256.Sum256([]byte(token))
	actual := hex.EncodeToString(hash[:])
	return subtle.ConstantTimeCompare([]byte(actual), []byte(stored)) == 1
}

func (s *Service) Revoke() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.persistLocked(storedState{}); err != nil {
		return err
	}
	s.state = storedState{}
	s.clearPairingLocked()
	return nil
}

func (s *Service) persistLocked(state storedState) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create connector state directory: %w", err)
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode connector state: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(s.path), "connector-*.tmp")
	if err != nil {
		return fmt.Errorf("create connector state file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("protect connector state file: %w", err)
	}
	if _, err := temp.Write(raw); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write connector state: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("flush connector state: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close connector state: %w", err)
	}
	if err := os.Rename(tempName, s.path); err != nil {
		_ = os.Remove(s.path)
		if retryErr := os.Rename(tempName, s.path); retryErr != nil {
			return fmt.Errorf("commit connector state: %w", retryErr)
		}
	}
	return nil
}

func (s *Service) clearPairingLocked() {
	s.pairCode = ""
	s.pairExpiresAt = time.Time{}
	s.pairFailures = 0
}
