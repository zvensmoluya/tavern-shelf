package importer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestChangedInboxSourceSurvivesCommit(t *testing.T) {
	imp, p, s := testImporter(t)
	source := filepath.Join(p.Inbox, "arriving.json")
	if err := os.WriteFile(source, []byte(validCard), 0600); err != nil {
		t.Fatal(err)
	}
	replacement := []byte(`{"spec":"chara_card_v2","data":{"name":"New arrival"}}`)
	imp.now = func() time.Time {
		if err := os.WriteFile(source, replacement, 0600); err != nil {
			t.Fatal(err)
		}
		return time.Now()
	}
	result, err := imp.Import(context.Background(), source)
	if err == nil {
		t.Fatal("changed Inbox source must be reported for retry")
	}
	managed, readErr := os.ReadFile(filepath.Join(p.Library, result.Character.SourceRelPath))
	if readErr != nil || string(managed) != validCard {
		t.Fatalf("committed snapshot lost: %v", readErr)
	}
	remaining, readErr := os.ReadFile(source)
	if readErr != nil || !bytes.Equal(remaining, replacement) {
		t.Fatalf("new source content lost: %v", readErr)
	}
	imp.now = time.Now
	second, err := imp.Import(context.Background(), source)
	if err != nil || second.Name != "New arrival" {
		t.Fatalf("changed content could not be imported on retry: %#v, %v", second, err)
	}
	characters, err := s.List(context.Background())
	if err != nil || len(characters) != 2 {
		t.Fatalf("both source versions must be retained: %d, %v", len(characters), err)
	}
}

func TestArchiveIdentifiedCardsWithoutRewriting(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  []byte
	}{
		{"No Cover.json", []byte(`{"spec":"chara_card_v3","data":{"name":"No Cover","tags":"one","character_version":3,"personality":{}}}`)},
		{"Missing Name.json", []byte(`{"spec":"chara_card_v2","data":{}}`)},
		{"Damaged.json", []byte(`{"spec":"chara_card_v3","data":`)},
		{"Readable.png", importPNG(t, `{"spec":"chara_card_v3","data":{"name":"PNG Card","tags":false}}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			imp, p, _ := testImporter(t)
			directory := t.TempDir()
			source := filepath.Join(directory, tc.name)
			if err := os.WriteFile(source, tc.raw, 0600); err != nil {
				t.Fatal(err)
			}
			result, err := imp.ImportFrom(context.Background(), directory, source)
			if err != nil || result.Kind != "character" || len(result.Character.Manifest.Warnings) == 0 {
				t.Fatalf("identified card rejected: %#v, %v", result, err)
			}
			if tc.name == "Missing Name.json" && result.Name != "Missing Name" {
				t.Fatalf("staging replaced the original display filename: %q", result.Name)
			}
			for _, filename := range []string{source, filepath.Join(p.Library, result.Character.SourceRelPath)} {
				actual, err := os.ReadFile(filename)
				if err != nil || !bytes.Equal(actual, tc.raw) {
					t.Fatalf("source bytes changed: %v", err)
				}
			}
			duplicate, err := imp.ImportFrom(context.Background(), directory, source)
			if err != nil || !duplicate.Duplicate {
				t.Fatalf("partial projection broke content deduplication: %v", err)
			}
		})
	}
}

func TestUnrecognizedPNGRemainsInInbox(t *testing.T) {
	imp, p, s := testImporter(t)
	var raw bytes.Buffer
	if err := png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(p.Inbox, "ordinary.png")
	if err := os.WriteFile(source, raw.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := imp.Import(context.Background(), source); err == nil {
		t.Fatal("ordinary image was identified as a card")
	}
	if after, err := os.ReadFile(source); err != nil || !bytes.Equal(after, raw.Bytes()) {
		t.Fatal("unrecognized source was changed")
	}
	if rows, _ := s.List(context.Background()); len(rows) != 0 {
		t.Fatal("unrecognized source entered the library")
	}
}

func importPNG(t *testing.T, json string) []byte {
	t.Helper()
	var base bytes.Buffer
	if err := png.Encode(&base, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	raw := base.Bytes()
	var result bytes.Buffer
	result.Write(raw[:len(raw)-12])
	payload := []byte("chara\x00" + base64.StdEncoding.EncodeToString([]byte(json)))
	_ = binary.Write(&result, binary.BigEndian, uint32(len(payload)))
	result.WriteString("tEXt")
	result.Write(payload)
	_ = binary.Write(&result, binary.BigEndian, crc32.ChecksumIEEE(append([]byte("tEXt"), payload...)))
	result.Write(raw[len(raw)-12:])
	return result.Bytes()
}
