package card

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openai/tavern-shelf/internal/manifest"
)

var (
	ErrUnsupported = errors.New("unsupported character card format")
	ErrInvalidPNG  = errors.New("invalid PNG character card")
)

const maxCardSize = 64 << 20

type Character struct {
	Name           string           `json:"name"`
	Creator        string           `json:"creator,omitempty"`
	Spec           string           `json:"spec,omitempty"`
	SpecVersion    string           `json:"specVersion,omitempty"`
	Tags           []string         `json:"tags"`
	HasWorldbook   bool             `json:"hasWorldbook"`
	HasRegex       bool             `json:"hasRegex"`
	HasExtensions  bool             `json:"hasExtensions"`
	HasInteractive bool             `json:"hasInteractive"`
	SourceFormat   string           `json:"sourceFormat"`
	SourceIsImage  bool             `json:"sourceIsImage"`
	Manifest       manifest.Content `json:"manifest"`
}

type envelope struct {
	Spec        string          `json:"spec"`
	SpecVersion string          `json:"spec_version"`
	Data        json.RawMessage `json:"data"`
}

type cardData struct {
	Name                     string                     `json:"name"`
	Nickname                 string                     `json:"nickname"`
	Creator                  string                     `json:"creator"`
	CharacterVersion         string                     `json:"character_version"`
	Description              string                     `json:"description"`
	Personality              string                     `json:"personality"`
	Scenario                 string                     `json:"scenario"`
	FirstMessage             string                     `json:"first_mes"`
	MessageExample           string                     `json:"mes_example"`
	CreatorNotes             string                     `json:"creator_notes"`
	CreatorComment           string                     `json:"creatorcomment"`
	CreatorNotesMultilingual map[string]string          `json:"creator_notes_multilingual"`
	SystemPrompt             string                     `json:"system_prompt"`
	PostHistoryInstructions  string                     `json:"post_history_instructions"`
	AlternateGreetings       []string                   `json:"alternate_greetings"`
	GroupOnlyGreetings       []string                   `json:"group_only_greetings"`
	CharacterBook            *characterBook             `json:"character_book"`
	Tags                     []string                   `json:"tags"`
	Extensions               map[string]json.RawMessage `json:"extensions"`
	Assets                   []asset                    `json:"assets"`
	Sources                  []string                   `json:"source"`
	CreationDate             int64                      `json:"creation_date"`
	ModificationDate         int64                      `json:"modification_date"`
}

type characterBook struct {
	Name              string               `json:"name"`
	Description       string               `json:"description"`
	ScanDepth         *int                 `json:"scan_depth"`
	TokenBudget       *int                 `json:"token_budget"`
	RecursiveScanning *bool                `json:"recursive_scanning"`
	Entries           []characterBookEntry `json:"entries"`
}

type characterBookEntry struct {
	Name           string   `json:"name"`
	Comment        string   `json:"comment"`
	Keys           []string `json:"keys"`
	SecondaryKeys  []string `json:"secondary_keys"`
	Content        string   `json:"content"`
	Enabled        bool     `json:"enabled"`
	Constant       bool     `json:"constant"`
	Selective      bool     `json:"selective"`
	UseRegex       bool     `json:"use_regex"`
	CaseSensitive  bool     `json:"case_sensitive"`
	InsertionOrder int      `json:"insertion_order"`
}

type asset struct {
	Type string `json:"type"`
	URI  string `json:"uri"`
	Name string `json:"name"`
	Ext  string `json:"ext"`
}

type regexScript struct {
	ScriptName    string `json:"scriptName"`
	Name          string `json:"name"`
	FindRegex     string `json:"findRegex"`
	ReplaceString string `json:"replaceString"`
	Placement     []int  `json:"placement"`
	Disabled      bool   `json:"disabled"`
	MarkdownOnly  bool   `json:"markdownOnly"`
	PromptOnly    bool   `json:"promptOnly"`
	RunOnEdit     bool   `json:"runOnEdit"`
	MinDepth      *int   `json:"minDepth"`
	MaxDepth      *int   `json:"maxDepth"`
}

func Supported(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json", ".png":
		return true
	default:
		return false
	}
}

func ParseFile(path string) (Character, error) {
	f, err := os.Open(path)
	if err != nil {
		return Character{}, fmt.Errorf("open card: %w", err)
	}
	defer f.Close()

	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return ParseJSON(io.LimitReader(f, maxCardSize))
	case ".png":
		return ParsePNG(io.LimitReader(f, maxCardSize))
	default:
		return Character{}, ErrUnsupported
	}
}

func ParseJSON(r io.Reader) (Character, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return Character{}, fmt.Errorf("read JSON card: %w", err)
	}
	return parseJSONBytes(raw, "json", false)
}

func parseJSONBytes(raw []byte, format string, image bool) (Character, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return Character{}, fmt.Errorf("decode character card JSON: %w", err)
	}
	var data cardData
	dataSource := raw
	if len(bytes.TrimSpace(env.Data)) > 0 && !bytes.Equal(bytes.TrimSpace(env.Data), []byte("null")) {
		dataSource = env.Data
	}
	if err := json.Unmarshal(dataSource, &data); err != nil {
		return Character{}, fmt.Errorf("decode character card data: %w", err)
	}
	if strings.TrimSpace(data.Name) == "" {
		return Character{}, errors.New("character card has no name")
	}
	if !image && !isCharacterEnvelope(env) && !hasLegacyCharacterFields(dataSource) {
		return Character{}, fmt.Errorf("%w: named JSON does not contain character card fields", ErrUnsupported)
	}
	content := buildManifest(data, raw)
	return Character{
		Name:           content.Character.Name,
		Creator:        content.Character.Creator,
		Spec:           strings.TrimSpace(env.Spec),
		SpecVersion:    strings.TrimSpace(env.SpecVersion),
		Tags:           content.Character.Tags,
		HasWorldbook:   content.CharacterBook != nil,
		HasRegex:       len(content.RegexScripts) > 0 || bookUsesRegex(content.CharacterBook),
		HasExtensions:  len(content.Extensions) > 0,
		HasInteractive: content.Interaction.HasHTML || content.Interaction.HasJavaScript || content.Interaction.HasInteractiveExtension,
		SourceFormat:   format,
		SourceIsImage:  image,
		Manifest:       content,
	}, nil
}

func isCharacterEnvelope(env envelope) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(env.Spec)), "chara_card_")
}

func hasLegacyCharacterFields(raw []byte) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return false
	}
	for _, key := range []string{
		"description", "personality", "scenario", "first_mes", "mes_example",
		"alternate_greetings", "character_book", "system_prompt", "post_history_instructions",
	} {
		if _, ok := fields[key]; ok {
			return true
		}
	}
	return false
}

func buildManifest(data cardData, raw []byte) manifest.Content {
	content := manifest.Content{
		SchemaVersion: manifest.CurrentSchemaVersion,
		Character: manifest.Character{
			Name: strings.TrimSpace(data.Name), Nickname: strings.TrimSpace(data.Nickname),
			Creator: strings.TrimSpace(data.Creator), CharacterVersion: strings.TrimSpace(data.CharacterVersion),
			Tags: cleanTags(data.Tags), Description: strings.TrimSpace(data.Description),
			Personality: strings.TrimSpace(data.Personality), Scenario: strings.TrimSpace(data.Scenario),
			MessageExample:           strings.TrimSpace(data.MessageExample),
			CreatorNotes:             strings.TrimSpace(firstNonEmpty(data.CreatorNotes, data.CreatorComment)),
			CreatorNotesMultilingual: cleanStringMap(data.CreatorNotesMultilingual),
			SystemPrompt:             strings.TrimSpace(data.SystemPrompt),
			PostHistoryInstructions:  strings.TrimSpace(data.PostHistoryInstructions),
		},
		Greetings: manifest.Greetings{
			FirstMessage: strings.TrimSpace(data.FirstMessage), Alternate: cleanStrings(data.AlternateGreetings),
			GroupOnly: cleanStrings(data.GroupOnlyGreetings),
		},
		RegexScripts: parseRegexScripts(data.Extensions),
		Extensions:   extensionManifest(data.Extensions),
		Assets:       assetManifest(data.Assets),
		Sources:      cleanStrings(data.Sources),
		Interaction:  detectInteraction(raw, data.Extensions),
		CreationDate: data.CreationDate,
		ModifiedDate: data.ModificationDate,
	}
	content.Greetings.AlternateCount = len(content.Greetings.Alternate)
	content.Greetings.GroupOnlyCount = len(content.Greetings.GroupOnly)
	content.Greetings.TotalCount = len(content.Greetings.Alternate) + len(content.Greetings.GroupOnly)
	if content.Greetings.FirstMessage != "" {
		content.Greetings.TotalCount++
	}
	if data.CharacterBook != nil {
		content.CharacterBook = bookManifest(*data.CharacterBook)
	}
	return content
}

func bookManifest(book characterBook) *manifest.CharacterBook {
	result := &manifest.CharacterBook{
		Name: strings.TrimSpace(book.Name), Description: strings.TrimSpace(book.Description),
		EntryCount: len(book.Entries), ScanDepth: book.ScanDepth, TokenBudget: book.TokenBudget,
		RecursiveScanning: book.RecursiveScanning,
		Entries:           make([]manifest.CharacterBookEntry, 0, len(book.Entries)),
	}
	for index, entry := range book.Entries {
		name := firstNonEmpty(entry.Name, entry.Comment)
		if name == "" && len(entry.Keys) > 0 {
			name = strings.Join(entry.Keys, " · ")
		}
		if name == "" {
			name = fmt.Sprintf("Entry %d", index+1)
		}
		result.Entries = append(result.Entries, manifest.CharacterBookEntry{
			Name: strings.TrimSpace(name), Comment: strings.TrimSpace(entry.Comment),
			Keys: cleanStrings(entry.Keys), SecondaryKeys: cleanStrings(entry.SecondaryKeys),
			Content: strings.TrimSpace(entry.Content), Enabled: entry.Enabled, Constant: entry.Constant,
			Selective: entry.Selective, UseRegex: entry.UseRegex, CaseSensitive: entry.CaseSensitive,
			InsertionOrder: entry.InsertionOrder,
		})
		if entry.Enabled {
			result.EnabledEntryCount++
		}
	}
	return result
}

func parseRegexScripts(extensions map[string]json.RawMessage) []manifest.RegexScript {
	raw, ok := extensions["regex_scripts"]
	if !ok {
		return []manifest.RegexScript{}
	}
	var scripts []regexScript
	if err := json.Unmarshal(raw, &scripts); err != nil {
		return []manifest.RegexScript{}
	}
	result := make([]manifest.RegexScript, 0, len(scripts))
	for index, script := range scripts {
		name := strings.TrimSpace(firstNonEmpty(script.ScriptName, script.Name))
		if name == "" {
			name = fmt.Sprintf("Regex %d", index+1)
		}
		result = append(result, manifest.RegexScript{
			Name: name, FindRegex: strings.TrimSpace(script.FindRegex), ReplaceString: script.ReplaceString,
			Placement: script.Placement, Disabled: script.Disabled, MarkdownOnly: script.MarkdownOnly,
			PromptOnly: script.PromptOnly, RunOnEdit: script.RunOnEdit,
			MinDepth: script.MinDepth, MaxDepth: script.MaxDepth,
		})
	}
	return result
}

func extensionManifest(extensions map[string]json.RawMessage) []manifest.Extension {
	keys := make([]string, 0, len(extensions))
	for key := range extensions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]manifest.Extension, 0, len(keys))
	for _, key := range keys {
		result = append(result, manifest.Extension{Name: key, Kind: jsonKind(extensions[key])})
	}
	return result
}

func assetManifest(assets []asset) []manifest.Asset {
	result := make([]manifest.Asset, 0, len(assets))
	for _, item := range assets {
		result = append(result, manifest.Asset{
			Type: strings.TrimSpace(item.Type), Name: strings.TrimSpace(item.Name),
			Ext: strings.TrimPrefix(strings.ToLower(strings.TrimSpace(item.Ext)), "."), URIKind: uriKind(item.URI),
		})
	}
	return result
}

func detectInteraction(raw []byte, extensions map[string]json.RawMessage) manifest.Interaction {
	lower := strings.ToLower(string(raw))
	result := manifest.Interaction{}
	htmlMarkers := []string{"<!doctype html", "<html", "<body", "<style", "<div", "<span", "<img", "<audio", "<video", "<button", "<iframe"}
	for _, marker := range htmlMarkers {
		if strings.Contains(lower, marker) {
			result.HasHTML = true
			result.Markers = append(result.Markers, "html")
			break
		}
	}
	javascriptMarkers := []string{"<script", "javascript:", "window.", "document.", "getelementbyid(", "addeventlistener("}
	for _, marker := range javascriptMarkers {
		if strings.Contains(lower, marker) {
			result.HasJavaScript = true
			result.Markers = append(result.Markers, "javascript")
			break
		}
	}
	for key := range extensions {
		keyLower := strings.ToLower(key)
		if strings.Contains(keyLower, "tavern_helper") || strings.Contains(keyLower, "tavernhelper") ||
			strings.Contains(keyLower, "risu") || strings.Contains(keyLower, "interactive") {
			result.HasInteractiveExtension = true
			result.Markers = append(result.Markers, "extension:"+key)
		}
	}
	return result
}

func bookUsesRegex(book *manifest.CharacterBook) bool {
	if book == nil {
		return false
	}
	for _, entry := range book.Entries {
		if entry.UseRegex {
			return true
		}
	}
	return false
}

func jsonKind(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "unknown"
	}
	switch trimmed[0] {
	case '{':
		return "object"
	case '[':
		return "array"
	case '"':
		return "string"
	case 't', 'f':
		return "boolean"
	case 'n':
		return "null"
	default:
		return "number"
	}
}

func uriKind(uri string) string {
	lower := strings.ToLower(strings.TrimSpace(uri))
	switch {
	case lower == "ccdefault:":
		return "default"
	case strings.HasPrefix(lower, "embeded://") || strings.HasPrefix(lower, "embedded://"):
		return "embedded"
	case strings.HasPrefix(lower, "data:"):
		return "data"
	case strings.HasPrefix(lower, "https://"):
		return "https"
	case strings.HasPrefix(lower, "http://"):
		return "http"
	case lower == "":
		return ""
	default:
		return "other"
	}
}

func ParsePNG(r io.Reader) (Character, error) {
	signature := make([]byte, 8)
	if _, err := io.ReadFull(r, signature); err != nil || !bytes.Equal(signature, []byte("\x89PNG\r\n\x1a\n")) {
		return Character{}, ErrInvalidPNG
	}
	payloads := make(map[string][]byte, 2)
	for {
		var length uint32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return Character{}, fmt.Errorf("read PNG chunk length: %w", err)
		}
		if length > maxCardSize {
			return Character{}, fmt.Errorf("PNG chunk is too large: %d bytes", length)
		}
		kind := make([]byte, 4)
		if _, err := io.ReadFull(r, kind); err != nil {
			return Character{}, fmt.Errorf("read PNG chunk type: %w", err)
		}
		data := make([]byte, length)
		if _, err := io.ReadFull(r, data); err != nil {
			return Character{}, fmt.Errorf("read PNG chunk data: %w", err)
		}
		var storedCRC uint32
		if err := binary.Read(r, binary.BigEndian, &storedCRC); err != nil {
			return Character{}, fmt.Errorf("read PNG chunk checksum: %w", err)
		}
		checksum := crc32.NewIEEE()
		_, _ = checksum.Write(kind)
		_, _ = checksum.Write(data)
		if checksum.Sum32() != storedCRC {
			return Character{}, fmt.Errorf("%w: corrupt %s chunk", ErrInvalidPNG, kind)
		}
		var keyword string
		var payload []byte
		switch string(kind) {
		case "tEXt":
			keyword, payload = splitText(data)
		case "iTXt":
			keyword, payload = splitInternationalText(data)
		}
		keyword = strings.ToLower(keyword)
		if (keyword == "chara" || keyword == "ccv3") && payloads[keyword] == nil {
			payloads[keyword] = payload
		}
		if string(kind) == "IEND" {
			break
		}
	}
	for _, keyword := range []string{"ccv3", "chara"} {
		payload, ok := payloads[keyword]
		if !ok {
			continue
		}
		decoded, err := decodePayload(payload)
		if err != nil {
			return Character{}, err
		}
		return parseJSONBytes(decoded, "png", true)
	}
	return Character{}, fmt.Errorf("%w: character metadata chunk not found", ErrInvalidPNG)
}

func splitText(data []byte) (string, []byte) {
	parts := bytes.SplitN(data, []byte{0}, 2)
	if len(parts) != 2 {
		return "", nil
	}
	return string(parts[0]), parts[1]
}

func splitInternationalText(data []byte) (string, []byte) {
	keywordEnd := bytes.IndexByte(data, 0)
	if keywordEnd < 0 || len(data) < keywordEnd+3 {
		return "", nil
	}
	keyword := string(data[:keywordEnd])
	rest := data[keywordEnd+1:]
	compressed, compressionMethod := rest[0], rest[1]
	if compressionMethod != 0 || compressed > 1 {
		return "", nil
	}
	rest = rest[2:]
	languageEnd := bytes.IndexByte(rest, 0)
	if languageEnd < 0 {
		return "", nil
	}
	rest = rest[languageEnd+1:]
	translatedEnd := bytes.IndexByte(rest, 0)
	if translatedEnd < 0 {
		return "", nil
	}
	payload := rest[translatedEnd+1:]
	if compressed == 0 {
		return keyword, payload
	}
	reader, err := zlib.NewReader(bytes.NewReader(payload))
	if err != nil {
		return "", nil
	}
	defer reader.Close()
	decoded, err := io.ReadAll(io.LimitReader(reader, maxCardSize))
	if err != nil {
		return "", nil
	}
	return keyword, decoded
}

func decodePayload(payload []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(payload)
	if json.Valid(trimmed) {
		return trimmed, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(string(trimmed))
	if err != nil {
		return nil, fmt.Errorf("decode PNG character metadata: %w", err)
	}
	return decoded, nil
}

func cleanTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		key := strings.ToLower(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, tag)
	}
	return result
}

func cleanStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func cleanStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		if key, value = strings.TrimSpace(key), strings.TrimSpace(value); key != "" && value != "" {
			result[key] = value
		}
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
