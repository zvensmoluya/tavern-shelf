package resource

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/openai/tavern-shelf/internal/library"
	"github.com/openai/tavern-shelf/internal/manifest"
)

var ErrUnsupported = errors.New("unsupported Shelf resource format")

const maxResourceSize = 64 << 20

type Parsed struct {
	Kind        string
	Subtype     string
	Name        string
	Description string
	Worldbook   *manifest.CharacterBook
	Preset      *manifest.Preset
}

type worldbookEnvelope struct {
	Entries     json.RawMessage `json:"entries"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
}

type worldInfoEntry struct {
	UID            int      `json:"uid"`
	Key            []string `json:"key"`
	Keys           []string `json:"keys"`
	KeySecondary   []string `json:"keysecondary"`
	SecondaryKeys  []string `json:"secondary_keys"`
	Comment        string   `json:"comment"`
	Name           string   `json:"name"`
	Content        string   `json:"content"`
	Constant       bool     `json:"constant"`
	Selective      bool     `json:"selective"`
	UseRegex       bool     `json:"use_regex"`
	CaseSensitive  bool     `json:"case_sensitive"`
	Order          int      `json:"order"`
	InsertionOrder int      `json:"insertion_order"`
	Disable        bool     `json:"disable"`
	Enabled        *bool    `json:"enabled"`
	DisplayIndex   int      `json:"displayIndex"`
}

type indexedEntry struct {
	key   string
	entry worldInfoEntry
}

func ParseFile(path string) (Parsed, error) {
	if strings.ToLower(filepath.Ext(path)) != ".json" {
		return Parsed{}, ErrUnsupported
	}
	file, err := os.Open(path)
	if err != nil {
		return Parsed{}, fmt.Errorf("open resource: %w", err)
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxResourceSize))
	if err != nil {
		return Parsed{}, fmt.Errorf("read resource JSON: %w", err)
	}
	return ParseJSON(raw, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
}

func ParseJSON(raw []byte, fallbackName string) (Parsed, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return Parsed{}, fmt.Errorf("decode resource JSON: %w", err)
	}
	if parsed, ok := parseWorldbook(raw, fallbackName); ok {
		return parsed, nil
	}
	if parsed, ok := parsePreset(object, fallbackName); ok {
		return parsed, nil
	}
	return Parsed{}, ErrUnsupported
}

func parseWorldbook(raw []byte, fallbackName string) (Parsed, bool) {
	var envelope worldbookEnvelope
	if json.Unmarshal(raw, &envelope) != nil || len(bytes.TrimSpace(envelope.Entries)) == 0 {
		return Parsed{}, false
	}
	entries, ok := decodeWorldbookEntries(envelope.Entries)
	if !ok {
		return Parsed{}, false
	}
	book := &manifest.CharacterBook{
		Name:        strings.TrimSpace(firstNonEmpty(envelope.Name, fallbackName)),
		Description: strings.TrimSpace(envelope.Description),
		Entries:     make([]manifest.CharacterBookEntry, 0, len(entries)),
	}
	for index, item := range entries {
		entry := item.entry
		keys := cleanStrings(append(append([]string(nil), entry.Key...), entry.Keys...))
		secondary := cleanStrings(append(append([]string(nil), entry.KeySecondary...), entry.SecondaryKeys...))
		name := firstNonEmpty(entry.Name, entry.Comment)
		if name == "" && len(keys) > 0 {
			name = strings.Join(keys, " · ")
		}
		if name == "" {
			name = fmt.Sprintf("Entry %d", index+1)
		}
		enabled := !entry.Disable
		if entry.Enabled != nil {
			enabled = *entry.Enabled
		}
		order := entry.Order
		if entry.InsertionOrder != 0 {
			order = entry.InsertionOrder
		}
		book.Entries = append(book.Entries, manifest.CharacterBookEntry{
			Name: strings.TrimSpace(name), Comment: strings.TrimSpace(entry.Comment),
			Keys: keys, SecondaryKeys: secondary, Content: strings.TrimSpace(entry.Content),
			Enabled: enabled, Constant: entry.Constant, Selective: entry.Selective,
			UseRegex: entry.UseRegex, CaseSensitive: entry.CaseSensitive, InsertionOrder: order,
		})
		if enabled {
			book.EnabledEntryCount++
		}
	}
	book.EntryCount = len(book.Entries)
	return Parsed{
		Kind: library.ResourceWorldbook, Name: book.Name, Description: book.Description, Worldbook: book,
	}, true
}

func decodeWorldbookEntries(raw json.RawMessage) ([]indexedEntry, bool) {
	var object map[string]worldInfoEntry
	if err := json.Unmarshal(raw, &object); err == nil && object != nil {
		entries := make([]indexedEntry, 0, len(object))
		for key, entry := range object {
			entries = append(entries, indexedEntry{key: key, entry: entry})
		}
		sort.SliceStable(entries, func(i, j int) bool {
			left, right := entries[i].entry.DisplayIndex, entries[j].entry.DisplayIndex
			if left != right {
				return left < right
			}
			leftUID, leftErr := strconv.Atoi(entries[i].key)
			rightUID, rightErr := strconv.Atoi(entries[j].key)
			if leftErr == nil && rightErr == nil {
				return leftUID < rightUID
			}
			return entries[i].key < entries[j].key
		})
		return entries, true
	}
	var array []worldInfoEntry
	if err := json.Unmarshal(raw, &array); err != nil || array == nil {
		return nil, false
	}
	entries := make([]indexedEntry, 0, len(array))
	for index, entry := range array {
		entries = append(entries, indexedEntry{key: strconv.Itoa(index), entry: entry})
	}
	return entries, true
}

type presetSignature struct {
	typeName string
	required [][]string
}

var presetSignatures = []presetSignature{
	{typeName: "openai", required: [][]string{{"prompts", "prompt_order"}, {"chat_completion_source", "openai_model"}}},
	{typeName: "instruct", required: [][]string{{"input_sequence", "output_sequence"}}},
	{typeName: "context", required: [][]string{{"story_string", "example_separator"}}},
	{typeName: "system-prompt", required: [][]string{{"name", "content", "post_history"}}},
	{typeName: "reasoning", required: [][]string{{"name", "prefix", "suffix", "separator"}}},
	{typeName: "textgen", required: [][]string{{"samplers", "sampler_priority"}, {"dry_multiplier", "rep_pen", "temp"}}},
	{typeName: "novel", required: [][]string{{"phrase_rep_pen", "order", "temperature"}}},
	{typeName: "kobold", required: [][]string{{"sampler_order", "rep_pen", "temp"}}},
}

var fieldLabels = map[string]string{
	"chat_completion_source": "API", "openai_model": "OpenAI model", "claude_model": "Claude model",
	"google_model": "Google model", "openrouter_model": "OpenRouter model", "temperature": "Temperature",
	"temp": "Temperature", "top_p": "Top P", "top_k": "Top K", "min_p": "Min P", "top_a": "Top A",
	"frequency_penalty": "Frequency penalty", "presence_penalty": "Presence penalty", "repetition_penalty": "Repetition penalty",
	"rep_pen": "Repetition penalty", "openai_max_context": "Context size", "max_context": "Context size",
	"openai_max_tokens": "Response tokens", "max_length": "Max length", "names_behavior": "Names behavior",
	"wrap": "Wrap sequences", "macro": "Enable macros", "skip_examples": "Skip examples",
	"use_stop_strings": "Use stop strings", "names_as_stop_strings": "Names as stop strings",
}

var textLabels = map[string]string{
	"content": "System prompt", "post_history": "Post-history prompt", "story_string": "Story string",
	"input_sequence": "Input sequence", "output_sequence": "Output sequence", "system_sequence": "System sequence",
	"stop_sequence": "Stop sequence", "input_suffix": "Input suffix", "output_suffix": "Output suffix",
	"system_suffix": "System suffix", "prefix": "Prefix", "suffix": "Suffix", "separator": "Separator",
}

func parsePreset(object map[string]json.RawMessage, fallbackName string) (Parsed, bool) {
	subtype := ""
	for _, signature := range presetSignatures {
		if matchesAnySignature(object, signature.required) {
			subtype = signature.typeName
			break
		}
	}
	if subtype == "" {
		return Parsed{}, false
	}
	name := strings.TrimSpace(readString(object["name"]))
	if name == "" {
		name = strings.TrimSpace(fallbackName)
	}
	preset := &manifest.Preset{Type: subtype, FieldCount: len(object), Fields: []manifest.PresetField{}, TextBlocks: []manifest.PresetField{}}
	if prompts, ok := object["prompts"]; ok {
		var values []json.RawMessage
		if json.Unmarshal(prompts, &values) == nil {
			preset.PromptCount = len(values)
		}
	}
	fieldKeys := make([]string, 0, len(fieldLabels))
	for key := range fieldLabels {
		fieldKeys = append(fieldKeys, key)
	}
	sort.Strings(fieldKeys)
	for _, key := range fieldKeys {
		if value, ok := primitiveValue(object[key]); ok {
			preset.Fields = append(preset.Fields, manifest.PresetField{Key: key, Label: fieldLabels[key], Value: value})
		}
	}
	textKeys := make([]string, 0, len(textLabels))
	for key := range textLabels {
		textKeys = append(textKeys, key)
	}
	sort.Strings(textKeys)
	for _, key := range textKeys {
		if value := strings.TrimSpace(readString(object[key])); value != "" {
			preset.TextBlocks = append(preset.TextBlocks, manifest.PresetField{Key: key, Label: textLabels[key], Value: value})
		}
	}
	description := presetDescription(subtype, preset)
	return Parsed{Kind: library.ResourcePreset, Subtype: subtype, Name: name, Description: description, Preset: preset}, true
}

func matchesAnySignature(object map[string]json.RawMessage, signatures [][]string) bool {
	for _, keys := range signatures {
		matches := true
		for _, key := range keys {
			if _, ok := object[key]; !ok {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

func primitiveValue(raw json.RawMessage) (string, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || trimmed[0] == '{' || trimmed[0] == '[' {
		return "", false
	}
	if trimmed[0] == '"' {
		value := readString(trimmed)
		return strings.TrimSpace(value), strings.TrimSpace(value) != ""
	}
	return string(trimmed), true
}

func readString(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return value
}

func presetDescription(subtype string, preset *manifest.Preset) string {
	labels := map[string]string{
		"openai": "Chat Completion preset", "instruct": "Instruct template", "context": "Context template",
		"system-prompt": "System prompt", "reasoning": "Reasoning template", "textgen": "Text generation preset",
		"novel": "NovelAI preset", "kobold": "Kobold preset",
	}
	description := labels[subtype]
	if preset.PromptCount > 0 {
		description += fmt.Sprintf(" · %d prompts", preset.PromptCount)
	}
	return description
}

func cleanStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
