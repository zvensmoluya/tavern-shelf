package connector

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestPairAuthorizePersistReplaceAndRevoke(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connector.json")
	service, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	pairing, err := service.BeginPairing()
	if err != nil || len(pairing.Code) != 6 {
		t.Fatalf("pairing = %#v, %v", pairing, err)
	}
	token, err := service.Pair(pairing.Code, "ST", "1.18.0")
	if err != nil || !service.Authorize(token) {
		t.Fatalf("pair token failed: %v", err)
	}
	reopened, err := Open(path)
	if err != nil || !reopened.Authorize(token) {
		t.Fatalf("persisted token unavailable: %v", err)
	}
	nextPairing, _ := reopened.BeginPairing()
	nextToken, err := reopened.Pair(nextPairing.Code, "ST 2", "1.18.0")
	if err != nil || reopened.Authorize(token) || !reopened.Authorize(nextToken) {
		t.Fatalf("token replacement failed: %v", err)
	}
	if err := reopened.Revoke(); err != nil || reopened.Authorize(nextToken) {
		t.Fatalf("revoke failed: %v", err)
	}
}

func TestPairingExpiresAndLocksAfterFailures(t *testing.T) {
	service, err := Open(filepath.Join(t.TempDir(), "connector.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	service.now = func() time.Time { return now }
	pairing, _ := service.BeginPairing()
	for index := 0; index < maxPairFailures; index++ {
		if _, err := service.Pair("wrong", "ST", ""); !errors.Is(err, ErrInvalidCode) {
			t.Fatalf("failure %d = %v", index, err)
		}
	}
	if _, err := service.Pair(pairing.Code, "ST", ""); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("locked code accepted: %v", err)
	}
	pairing, _ = service.BeginPairing()
	now = now.Add(pairingLifetime + time.Second)
	if _, err := service.Pair(pairing.Code, "ST", ""); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("expired code accepted: %v", err)
	}
}
