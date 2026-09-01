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
    "character_version":"1.4",
    "tags":["fantasy","Fantasy"," scholar "],
    "description":"An archivist of impossible places.",
    "personality":"Patient and curious.",
    "scenario":"The moon archive is opening.",
    "first_mes":"The doors remember you.",
    "alternate_greetings":["You came back.","The archive is quiet tonight."],
    "mes_example":"{{char}}: Every map is a promise.",
    "creator_notes":"Built for slow mysteries.",
    "system_prompt":"Stay in character.",
    "post_history_instructions":"Keep the archive consistent.",
    "character_book":{"name":"Moon Archive","entries":[
      {"name":"The stacks","keys":["archive","stacks"],"content":"Shelves move at night.","enabled":true,"insertion_order":10},
      {"comment":"The sealed gate","keys":["/gate/i"],"content":"The gate is sealed.","enabled":false,"insertion_order":20,"use_regex":true}
    ]},
    "extensions":{"regex_scripts":[{"scriptName":"Trim status","findRegex":"/<status>.*?<\\/status>/s","replaceString":"","placement":[1,2],"promptOnly":true}],"tavern_helper":{"panel":"<div><script>window.openArchive()</script></div>"}}
  }
}`

const v3Card = `{
  "spec":"chara_card_v3",
  "spec_version":"3.0",
  "data":{
    "name":"Selene",
    "nickname":"Selen",
    "creator":"Cartographer",
    "character_version":"3.1",
    "description":"A navigator between lost cities.",
    "first_mes":"Choose a horizon.",
    "alternate_greetings":["The compass is awake."],
    "group_only_greetings":["All of you, stay close."],
    "tags":["adventure"],
    "extensions":{},
    "assets":[{"type":"icon","uri":"ccdefault:","name":"main","ext":"png"},{"type":"background","uri":"embeded://assets/background/images/city.webp","name":"main","ext":"webp"}],
    "source":["https://example.test/cards/selene"],
    "creation_date":1700000000,
    "modification_date":1700001000
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
	content := character.Manifest
	if content.Character.Description == "" || content.Character.CharacterVersion != "1.4" {
		t.Fatalf("character content was not parsed: %#v", content.Character)
	}
	if content.Greetings.TotalCount != 3 || content.Greetings.AlternateCount != 2 {
		t.Fatalf("greetings were not counted: %#v", content.Greetings)
	}
	if content.CharacterBook == nil || content.CharacterBook.EntryCount != 2 || content.CharacterBook.Entries[1].Name != "The sealed gate" {
		t.Fatalf("character book manifest is incomplete: %#v", content.CharacterBook)
	}
	if len(content.RegexScripts) != 1 || content.RegexScripts[0].Name != "Trim status" || content.RegexScripts[0].FindRegex == "" {
		t.Fatalf("regex manifest is incomplete: %#v", content.RegexScripts)
	}
	if !content.Interaction.HasHTML || !content.Interaction.HasJavaScript || !content.Interaction.HasInteractiveExtension {
		t.Fatalf("interactive content was not detected: %#v", content.Interaction)
	}
}

func TestParseJSONV3StandardFields(t *testing.T) {
	character, err := ParseJSON(bytes.NewBufferString(v3Card))
	if err != nil {
		t.Fatal(err)
	}
	content := character.Manifest
	if character.Spec != "chara_card_v3" || content.Character.Nickname != "Selen" {
		t.Fatalf("V3 identity was not parsed: %#v", character)
	}
	if content.Greetings.GroupOnlyCount != 1 || content.Greetings.TotalCount != 3 {
		t.Fatalf("V3 greetings were not parsed: %#v", content.Greetings)
	}
	if len(content.Assets) != 2 || content.Assets[1].URIKind != "embedded" {
		t.Fatalf("V3 assets were not parsed: %#v", content.Assets)
	}
	if len(content.Sources) != 1 || content.CreationDate != 1700000000 || content.ModifiedDate != 1700001000 {
		t.Fatalf("V3 provenance was not parsed: %#v", content)
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
	raw := makePNGCard(t, pngTextChunk{"tEXt", append([]byte("chara\x00"), []byte(base64.StdEncoding.EncodeToString([]byte(v2Card)))...)})
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
	raw := makePNGCard(t, pngTextChunk{"iTXt", payload})
	character, err := ParsePNG(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if character.Name != "Mara" {
		t.Fatalf("unexpected PNG character: %#v", character)
	}
}

func TestParsePNGPrefersV3Chunk(t *testing.T) {
	chara := append([]byte("chara\x00"), []byte(base64.StdEncoding.EncodeToString([]byte(v2Card)))...)
	ccv3 := append([]byte("ccv3\x00"), []byte(base64.StdEncoding.EncodeToString([]byte(v3Card)))...)
	raw := makePNGCard(t, pngTextChunk{"tEXt", chara}, pngTextChunk{"tEXt", ccv3})
	character, err := ParsePNG(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if character.Name != "Selene" || character.Spec != "chara_card_v3" {
		t.Fatalf("V3 chunk did not take precedence: %#v", character)
	}
}

type pngTextChunk struct {
	kind    string
	payload []byte
}

func makePNGCard(t *testing.T, chunks ...pngTextChunk) []byte {
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
	result := append([]byte{}, raw[:position]...)
	for _, item := range chunks {
		var chunk bytes.Buffer
		_ = binary.Write(&chunk, binary.BigEndian, uint32(len(item.payload)))
		chunk.WriteString(item.kind)
		chunk.Write(item.payload)
		checksum := crc32.NewIEEE()
		_, _ = checksum.Write([]byte(item.kind))
		_, _ = checksum.Write(item.payload)
		_ = binary.Write(&chunk, binary.BigEndian, checksum.Sum32())
		result = append(result, chunk.Bytes()...)
	}
	result = append(result, raw[position:]...)
	return result
}
