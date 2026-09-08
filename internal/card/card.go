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
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zvensmoluya/tavern-shelf/internal/manifest"
)

var (
	ErrUnsupported = errors.New("unsupported character card format")
	ErrInvalidPNG  = errors.New("invalid PNG character card")
)

const MaxSourceSize int64 = 64 << 20
const maxCardSize = int(MaxSourceSize)

type Character struct {
	nameMissing    bool
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
	return ParseFileWithName(path, filepath.Base(path))
}

// ParseFileWithName keeps the original display filename when rebuilding a
// manifest from a managed file whose storage name is always source.png/json.
func ParseFileWithName(path, sourceName string) (Character, error) {
	f, err := os.Open(path)
	if err != nil {
		return Character{}, fmt.Errorf("open card: %w", err)
	}
	defer f.Close()

	if !Supported(path) {
		return Character{}, ErrUnsupported
	}
	raw, err := readCardBytes(f)
	if err != nil {
		return Character{}, err
	}
	var result Character
	if bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n")) {
		result, err = parsePNGBytes(raw)
	} else {
		result, err = parseJSONBytes(raw, "json", false)
	}
	if err == nil && result.nameMissing {
		if name := strings.TrimSpace(strings.TrimSuffix(filepath.Base(sourceName), filepath.Ext(sourceName))); name != "" {
			result.Name = name
			result.Manifest.Character.Name = name
		}
	}
	return result, err
}

func ParseJSON(r io.Reader) (Character, error) {
	raw, err := readCardBytes(r)
	if err != nil {
		return Character{}, fmt.Errorf("read JSON card: %w", err)
	}
	return parseJSONBytes(raw, "json", false)
}

func parseJSONBytes(raw []byte, format string, image bool) (Character, error) {
	raw = bytes.TrimPrefix(bytes.TrimSpace(raw), []byte{0xef, 0xbb, 0xbf})
	warnings := []string{}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		if !image && !hasReadableCardSpec(raw) {
			return Character{}, fmt.Errorf("decode character card JSON: %w", err)
		}
		warnings = append(warnings, "角色卡内容未能完整解析；原始文件已保留，可交给播放器处理。")
	}
	var env envelope
	if fields != nil {
		projectJSON(raw, &env, "", &warnings)
	}
	var data cardData
	dataSource := raw
	if len(bytes.TrimSpace(env.Data)) > 0 && !bytes.Equal(bytes.TrimSpace(env.Data), []byte("null")) {
		dataSource = env.Data
	}
	if !image && !isCharacterEnvelope(env) && !hasReadableCardSpec(raw) && !hasLegacyCharacterFields(dataSource) {
		return Character{}, fmt.Errorf("%w: named JSON does not contain character card fields", ErrUnsupported)
	}
	if fields != nil {
		projectJSON(dataSource, &data, "data", &warnings)
	}
	nameMissing := strings.TrimSpace(data.Name) == ""
	if nameMissing {
		data.Name = "未命名角色卡"
		warnings = append(warnings, "未能读取角色名称，使用文件名展示；原始文件已保留。")
	}
	content := buildManifest(data, raw, &warnings)
	content.Warnings = warnings
	return Character{
		nameMissing:    nameMissing,
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
	if _, ok := fields["name"]; !ok {
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

func buildManifest(data cardData, raw []byte, warnings *[]string) manifest.Content {
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
		RegexScripts: parseRegexScripts(data.Extensions, warnings),
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

func parseRegexScripts(extensions map[string]json.RawMessage, warnings *[]string) []manifest.RegexScript {
	raw, ok := extensions["regex_scripts"]
	if !ok {
		return []manifest.RegexScript{}
	}
	var scripts []regexScript
	projectJSON(raw, &scripts, "data.extensions.regex_scripts", warnings)
	result := make([]manifest.RegexScript, 0, len(scripts))
	for index, script := range scripts {
		if script.Placement == nil {
			script.Placement = []int{}
		}
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
	raw, err := readCardBytes(r)
	if err != nil {
		return Character{}, err
	}
	return parsePNGBytes(raw)
}

func parsePNGBytes(raw []byte) (Character, error) {
	if len(raw) < 8 || !bytes.Equal(raw[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return Character{}, ErrInvalidPNG
	}
	payloads := make(map[string][][]byte, 2)
	warnings := []string{}
	hasImageData, ended := false, false
	for position := 8; position < len(raw); {
		if len(raw)-position < 12 {
			warnings = append(warnings, "PNG 的尾部不完整；已保留原始文件。")
			break
		}
		length := uint64(binary.BigEndian.Uint32(raw[position:]))
		if length > uint64(len(raw)-position-12) {
			warnings = append(warnings, "PNG 的部分数据不完整；已保留原始文件。")
			// A readable card keyword is still an identity signal in a truncated chunk.
			kind := string(raw[position+4 : position+8])
			if kind == "tEXt" || kind == "iTXt" || kind == "zTXt" {
				key, _ := splitText(raw[position+8:])
				key = strings.ToLower(key)
				if key == "chara" || key == "ccv3" {
					payloads[key] = append(payloads[key], nil)
				}
			}
			break
		}
		end := position + 8 + int(length)
		kind := string(raw[position+4 : position+8])
		data := raw[position+8 : end]
		if crc32.ChecksumIEEE(raw[position+4:end]) != binary.BigEndian.Uint32(raw[end:]) && len(warnings) < 64 {
			warnings = append(warnings, "PNG 的部分数据校验不一致；已保留原始文件。")
		}
		if kind == "IDAT" {
			hasImageData = true
		}
		if kind == "tEXt" || kind == "iTXt" || kind == "zTXt" {
			keyword, payload := splitText(data)
			keyword = strings.ToLower(keyword)
			if keyword == "chara" || keyword == "ccv3" {
				switch kind {
				case "iTXt":
					_, payload = splitInternationalText(data)
				case "zTXt":
					payload = inflateText(payload)
				}
				payloads[keyword] = append(payloads[keyword], payload)
			}
		}
		position = end + 4
		if kind == "IEND" {
			ended = true
			break
		}
	}
	if len(payloads) == 0 {
		return Character{}, fmt.Errorf("%w: character metadata chunk not found", ErrInvalidPNG)
	}
	if !ended {
		warnings = append(warnings, "PNG 缺少结束标记；已保留原始文件。")
	}
	var result Character
	found := false
	for _, keyword := range []string{"ccv3", "chara"} {
		for _, payload := range payloads[keyword] {
			decoded, err := decodePayload(payload)
			if err != nil || !json.Valid(bytes.TrimPrefix(bytes.TrimSpace(decoded), []byte{0xef, 0xbb, 0xbf})) {
				if len(warnings) < 64 {
					warnings = append(warnings, "部分角色卡内容未能解析；原始文件已完整保留。")
				}
				continue
			}
			result, err = parseJSONBytes(decoded, "png", true)
			if err == nil {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		result, _ = parseJSONBytes(nil, "png", true)
	}
	_, coverErr := png.DecodeConfig(bytes.NewReader(raw))
	result.SourceIsImage = coverErr == nil && hasImageData
	if !result.SourceIsImage {
		warnings = append(warnings, "封面暂时无法显示；角色卡原始文件已保留。")
	}
	result.Manifest.Warnings = append(result.Manifest.Warnings, warnings...)
	return result, nil
}

// zTXt uses a compression method byte followed by a zlib stream.
func inflateText(data []byte) []byte {
	if len(data) < 2 || data[0] != 0 {
		return nil
	}
	reader, err := zlib.NewReader(bytes.NewReader(data[1:]))
	if err != nil {
		return nil
	}
	defer reader.Close()
	decoded, err := readCardBytes(reader)
	if err != nil {
		return nil
	}
	return decoded
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
	decoded, err := readCardBytes(reader)
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
