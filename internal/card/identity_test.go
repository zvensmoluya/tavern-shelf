package card

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestContentFingerprintPreservesUnknownData(t *testing.T) {
	a := `{"spec":"chara_card_v2","data":{"name":"A","creator":"B","character_version":"1.0","extensions":{"unknown":{"n":9007199254740993,"items":[1,2]}}}}`
	b := ` { "data": {"extensions":{"unknown":{"items":[1,2],"n":9007199254740993}},"creator":"B","character_version":"1.0","name":"A"}, "spec":"chara_card_v2" } `
	if contentFingerprint([]byte(a)) == "" || contentFingerprint([]byte(a)) != contentFingerprint([]byte(b)) {
		t.Fatal("formatting or object order changed identity")
	}
	for _, changed := range []string{strings.Replace(a, "9007199254740993", "9007199254740992", 1), strings.Replace(a, "[1,2]", "[2,1]", 1), strings.Replace(a, `"A"`, `"A "`, 1)} {
		if contentFingerprint([]byte(changed)) == contentFingerprint([]byte(a)) {
			t.Fatal("meaningful data change was lost")
		}
	}
	for _, ambiguous := range []string{`{"name":"A","name":"B"}`, `{"a":{"x":1,"x":2}}`, `{"name":`, `null`, `{} {}`} {
		if contentFingerprint([]byte(ambiguous)) != "" {
			t.Fatalf("ambiguous JSON fingerprinted: %s", ambiguous)
		}
	}
}

func TestPayloadIdentityIndependentOfContainerAndCover(t *testing.T) {
	jsonCard, err := ParseJSON(strings.NewReader(v2Card))
	if err != nil {
		t.Fatal(err)
	}
	raw := makePNGCard(t, pngTextChunk{kind: "tEXt", payload: []byte("chara\x00" + v2Card)})
	pngCard, err := parsePNGBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if pngCard.ContentHash != jsonCard.ContentHash || pngCard.CoverHash == "" {
		t.Fatal("container changed data identity or cover missing")
	}
	other := makePNGCard(t, pngTextChunk{kind: "tEXt", payload: []byte("chara\x00" + strings.Replace(v2Card, "impossible places", "possible places", 1))})
	otherCard, err := parsePNGBytes(other)
	if err != nil {
		t.Fatal(err)
	}
	if otherCard.CoverHash != pngCard.CoverHash || otherCard.ContentHash == pngCard.ContentHash {
		t.Fatal("cover and content identity are coupled")
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var cover bytes.Buffer
	if err := png.Encode(&cover, img); err != nil {
		t.Fatal(err)
	}
	if coverFingerprint(cover.Bytes()) == pngCard.CoverHash {
		t.Fatal("different pixels share a cover identity")
	}
}

func TestFingerprintDepthIsBounded(t *testing.T) {
	raw := `{"nested":` + strings.Repeat("[", 300) + "0" + strings.Repeat("]", 300) + "}"
	if contentFingerprint([]byte(raw)) != "" {
		t.Fatal("excessively nested JSON was classified")
	}
}

func TestMissingIdentityDoesNotUseFilename(t *testing.T) {
	for _, raw := range []string{`{"spec":"chara_card_v2","data":{"creator":"B"}}`, `{"spec":"chara_card_v2","data":{"name":"A"}}`} {
		parsed, err := ParseJSON(strings.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if parsed.IdentityKey != "" {
			t.Fatal("incomplete identity used for automatic grouping")
		}
	}
}
