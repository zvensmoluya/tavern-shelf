package resource

import (
	"errors"
	"testing"

	"github.com/zvensmoluya/tavern-shelf/internal/library"
)

func TestParseStandaloneSillyTavernWorldbook(t *testing.T) {
	raw := []byte(`{"entries":{"1":{"uid":1,"key":["rain","storm"],"keysecondary":["weather"],"comment":"Rainfall","content":"It rains every evening.","constant":false,"selective":true,"order":90,"disable":false,"displayIndex":1},"0":{"uid":0,"key":["city"],"comment":"The City","content":"A city on a cliff.","constant":true,"order":100,"disable":true,"displayIndex":0}}}`)
	parsed, err := ParseJSON(raw, "时雨的童话世界")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Kind != library.ResourceWorldbook || parsed.Name != "时雨的童话世界" || parsed.Worldbook == nil {
		t.Fatalf("unexpected worldbook: %#v", parsed)
	}
	if parsed.Worldbook.EntryCount != 2 || parsed.Worldbook.EnabledEntryCount != 1 {
		t.Fatalf("unexpected worldbook counts: %#v", parsed.Worldbook)
	}
	if parsed.Worldbook.Entries[0].Name != "The City" || parsed.Worldbook.Entries[0].Enabled {
		t.Fatalf("worldbook ordering or enabled projection mismatch: %#v", parsed.Worldbook.Entries)
	}
}

func TestCharacterEmbeddedWorldbookIsNotStandaloneResource(t *testing.T) {
	raw := []byte(`{"spec":"chara_card_v2","data":{"name":"Mara","character_book":{"name":"Mara lore","entries":[{"keys":["mara"],"content":"Lore","enabled":true}]}}}`)
	if _, err := ParseJSON(raw, "mara"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("character card was misclassified as standalone resource: %v", err)
	}
}

func TestParsePresetTypes(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		subtype string
	}{
		{"openai", `{"chat_completion_source":"openai","openai_model":"gpt-4","temperature":1,"prompts":[],"prompt_order":[]}`, "openai"},
		{"instruct", `{"name":"ChatML","input_sequence":"<user>","output_sequence":"<assistant>","stop_sequence":"</s>"}`, "instruct"},
		{"context", `{"name":"Roleplay","story_string":"{{description}}","example_separator":"<START>"}`, "context"},
		{"system prompt", `{"name":"Immersive","content":"Write vividly.","post_history":""}`, "system-prompt"},
		{"reasoning", `{"name":"Think","prefix":"<think>","suffix":"</think>","separator":"\\n"}`, "reasoning"},
		{"textgen", `{"temp":0.8,"rep_pen":1.1,"dry_multiplier":0.2,"samplers":[],"sampler_priority":[]}`, "textgen"},
		{"novel", `{"temperature":1,"phrase_rep_pen":"medium","order":[0,1]}`, "novel"},
		{"kobold", `{"temp":0.7,"rep_pen":1.2,"sampler_order":[6,0,1]}`, "kobold"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := ParseJSON([]byte(test.raw), "Community Preset")
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Kind != library.ResourcePreset || parsed.Subtype != test.subtype || parsed.Name == "" || parsed.Preset == nil {
				t.Fatalf("unexpected preset: %#v", parsed)
			}
		})
	}
}

func TestUnknownJSONIsRejected(t *testing.T) {
	if _, err := ParseJSON([]byte(`{"theme":"dark","layout":"wide"}`), "settings"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unknown JSON should be rejected, got %v", err)
	}
}
