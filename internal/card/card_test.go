package card

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"testing"
)

const v2Card = `{
  "spec":"chara_card_v2",
  "spec_version":"2.0",
  "data":{
    "name":"Mara",
    "creator":"Inkkeeper",
    "tags":["fantasy","Fantasy"," scholar "],
    "character_book":{"entries":[]},
    "extensions":{"regex_scripts":[{"script":"x"}],"tavern_helper":{"enabled":true}}
  }
}`

func TestParseJSONV2(t *testing.T) {
	character, err := ParseJSON(bytes.NewBufferString(v2Card))
	if err != nil {
		t.Fatal(err)
	}
	if character.Name != "Mara" || character.Creator != "Inkkeeper" {
		t.Fatalf("unexpected identity: %#v", character)
	}
	if character.Spec != "chara_card_v2" || character.SpecVersion != "2.0" {
		t.Fatalf("unexpected spec: %#v", character)
	}
	if len(character.Tags) != 2 || character.Tags[1] != "scholar" {
		t.Fatalf("tags were not cleaned: %#v", character.Tags)
	}
	if !character.HasWorldbook || !character.HasRegex || !character.HasExtensions || !character.HasInteractive {
		t.Fatalf("features were not detected: %#v", character)
	}
}

func TestParseJSONV1(t *testing.T) {
	character, err := ParseJSON(bytes.NewBufferString(`{"name":"Old Friend","creator":"Anon","tags":["v1"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if character.Name != "Old Friend" || character.SourceFormat != "json" {
		t.Fatalf("unexpected card: %#v", character)
	}
}

func TestParseJSONRequiresName(t *testing.T) {
	if _, err := ParseJSON(bytes.NewBufferString(`{"spec":"chara_card_v2","data":{}}`)); err == nil {
		t.Fatal("expected nameless card to fail")
	}
}

func TestParsePNGTextCard(t *testing.T) {
	raw := makePNGCard(t, "tEXt", append([]byte("chara\x00"), []byte(base64.StdEncoding.EncodeToString([]byte(v2Card)))...))
	character, err := ParsePNG(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if character.Name != "Mara" || !character.SourceIsImage || character.SourceFormat != "png" {
		t.Fatalf("unexpected PNG character: %#v", character)
	}
}

func TestParsePNGInternationalV3Card(t *testing.T) {
	payload := append([]byte("ccv3\x00\x00\x00\x00\x00"), []byte(v2Card)...)
	raw := makePNGCard(t, "iTXt", payload)
	character, err := ParsePNG(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if character.Name != "Mara" {
		t.Fatalf("unexpected PNG character: %#v", character)
	}
}

func makePNGCard(t *testing.T, kind string, payload []byte) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 180, G: 110, B: 60, A: 255})
	var base bytes.Buffer
	if err := png.Encode(&base, img); err != nil {
		t.Fatal(err)
	}
	raw := base.Bytes()
	position := bytes.LastIndex(raw, []byte("IEND")) - 4
	if position < 8 {
		t.Fatal("encoded PNG has no IEND chunk")
	}
	var chunk bytes.Buffer
	_ = binary.Write(&chunk, binary.BigEndian, uint32(len(payload)))
	chunk.WriteString(kind)
	chunk.Write(payload)
	checksum := crc32.NewIEEE()
	_, _ = checksum.Write([]byte(kind))
	_, _ = checksum.Write(payload)
	_ = binary.Write(&chunk, binary.BigEndian, checksum.Sum32())
	result := append([]byte{}, raw[:position]...)
	result = append(result, chunk.Bytes()...)
	result = append(result, raw[position:]...)
	return result
}
