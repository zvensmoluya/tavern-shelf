package card

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompressedTextCard(t *testing.T) {
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	_, _ = writer.Write([]byte(base64.StdEncoding.EncodeToString([]byte(v2Card))))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	for _, chunk := range []pngTextChunk{
		{"zTXt", append([]byte("chara\x00\x00"), compressed.Bytes()...)},
		{"iTXt", append([]byte("chara\x00\x01\x00\x00\x00"), compressed.Bytes()...)},
	} {
		parsed, err := ParsePNG(bytes.NewReader(makePNGCard(t, chunk)))
		if err != nil || parsed.Name == "" || parsed.nameMissing {
			t.Fatalf("%s metadata was not projected: %v", chunk.kind, err)
		}
	}
}

func TestProjectionPreservesReadableFields(t *testing.T) {
	raw := `{"spec":"chara_card_v3","spec_version":3,"data":{"name":"Archive Me","character_version":12,"tags":"fantasy","description":"Keep this description","creation_date":"1700000000","personality":{},"character_book":{"entries":[{"content":"First","keys":"key","enabled":"true"},false,{"content":"Second","insertion_order":"10"}]},"extensions":{"regex_scripts":[{"scriptName":"Keep this script","placement":[1,"2",{}]},"bad entry",{"scriptName":"Another script"}],"unknown":{"anything":[false,3]}}}}`
	parsed, err := ParseJSON(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	content := parsed.Manifest
	if parsed.SpecVersion != "3" || parsed.Name != "Archive Me" || content.Character.CharacterVersion != "12" || content.Character.Description != "Keep this description" || content.Character.Personality != "" {
		t.Fatalf("readable fields were discarded: %#v", content.Character)
	}
	if len(parsed.Tags) != 1 || parsed.Tags[0] != "fantasy" || content.CreationDate != 1700000000 {
		t.Fatal("deterministic scalar projection failed")
	}
	if content.CharacterBook == nil || content.CharacterBook.EntryCount != 2 || !content.CharacterBook.Entries[0].Enabled || content.CharacterBook.Entries[1].Content != "Second" {
		t.Fatalf("worldbook siblings lost: %#v", content.CharacterBook)
	}
	if len(content.RegexScripts) != 2 || len(content.RegexScripts[0].Placement) != 2 || content.RegexScripts[1].Placement == nil {
		t.Fatalf("regex siblings lost: %#v", content.RegexScripts)
	}
	if len(content.Warnings) == 0 || len(content.Extensions) != 2 {
		t.Fatal("partial projection or opaque extensions not recorded")
	}
}

func TestDamagedJSONNeedsExplicitCardIdentity(t *testing.T) {
	for _, tc := range []struct {
		raw      string
		accepted bool
	}{
		{`{"spec":"chara_card_v3","data":{"name":`, true},
		{`{"spec":"chara_card_v2","data":false}`, true},
		{`{"spec":"chara_card_v3"}`, true},
		{`{"data":`, false},
		{`{"name":"Account","email":"private@example.test"}`, false},
		{`{"note":"spec: chara_card_v3","broken":`, false},
		{`{"nested":{"spec":"chara_card_v3"},"broken":`, false},
		{`{"description":"A general document"}`, false},
		{`["chara_card_v3"]`, false},
	} {
		parsed, err := ParseJSON(strings.NewReader(tc.raw))
		if (err == nil) != tc.accepted {
			t.Errorf("acceptance for %q = %v", tc.raw, err)
		}
		if tc.accepted && (parsed.Name == "" || len(parsed.Manifest.Warnings) == 0) {
			t.Errorf("missing fallback or warning for %q", tc.raw)
		}
	}
}

func TestPNGCardWithDamagedCoverOrMetadataIsPreserved(t *testing.T) {
	payload := []byte("chara\x00" + base64.StdEncoding.EncodeToString([]byte(v2Card)))
	normal := makePNGCard(t, pngTextChunk{"tEXt", payload})
	badCRC := append([]byte(nil), normal...)
	badCRC[len(badCRC)-1] ^= 1
	var withoutCover bytes.Buffer
	withoutCover.WriteString("\x89PNG\r\n\x1a\n")
	for _, chunk := range []pngTextChunk{{"tEXt", payload}, {"IEND", nil}} {
		_ = binary.Write(&withoutCover, binary.BigEndian, uint32(len(chunk.payload)))
		withoutCover.WriteString(chunk.kind)
		withoutCover.Write(chunk.payload)
		_ = binary.Write(&withoutCover, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(chunk.kind), chunk.payload...)))
	}
	for _, tc := range []struct {
		name  string
		raw   []byte
		cover bool
	}{
		{"bad CRC", badCRC, true},
		{"missing IEND", normal[:len(normal)-12], true},
		{"missing image", withoutCover.Bytes(), false},
		{"unreadable metadata", makePNGCard(t, pngTextChunk{"tEXt", []byte("chara\x00not-base64")}), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := ParsePNG(bytes.NewReader(tc.raw))
			if err != nil || parsed.Name == "" || parsed.SourceIsImage != tc.cover || len(parsed.Manifest.Warnings) == 0 {
				t.Fatalf("card was not preserved: %#v, %v", parsed, err)
			}
		})
	}
	if _, err := ParsePNG(bytes.NewReader(makePNGCard(t))); err == nil {
		t.Fatal("ordinary PNG must not be classified as a card")
	}
}

func TestPNGUsesReadableFallbackWithoutDiscardingSource(t *testing.T) {
	payload := []byte("chara\x00" + base64.StdEncoding.EncodeToString([]byte(v2Card)))
	raw := makePNGCard(t, pngTextChunk{"tEXt", []byte("ccv3\x00broken")}, pngTextChunk{"tEXt", payload})
	parsed, err := ParsePNG(bytes.NewReader(raw))
	if err != nil || parsed.Name != "Mara" || len(parsed.Manifest.Warnings) == 0 {
		t.Fatalf("readable alternate metadata not projected: %#v, %v", parsed, err)
	}
}

func TestFileNameFallbackAndContentDetection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.png")
	raw := []byte("\xef\xbb\xbf" + `{"spec":"chara_card_v3","data":{}}`)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseFileWithName(path, "Collected Card.json")
	if err != nil || parsed.Name != "Collected Card" || parsed.SourceFormat != "json" || parsed.SourceIsImage {
		t.Fatalf("identity depends on cover or managed filename: %#v, %v", parsed, err)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(raw, after) {
		t.Fatal("source was rewritten")
	}
}

func TestPNGSizeLimitIncludesTrailingBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write(makePNGCard(t, pngTextChunk{"tEXt", []byte("chara\x00" + v2Card)}))
	if err == nil {
		err = f.Truncate(MaxSourceSize + 1)
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("prepare source: %v, %v", err, closeErr)
	}
	if _, err := ParseFile(path); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("source size limit was bypassed: %v", err)
	}
}
