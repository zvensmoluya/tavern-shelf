package adaptation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/zvensmoluya/tavern-shelf/internal/card"
)

const ProgramViewSchemaVersion = 1

type ProgramView struct {
	SchemaVersion        int                 `json:"schemaVersion"`
	SourceSHA256         string              `json:"sourceSha256"`
	ProgramBlocks        []ProgramBlock      `json:"programBlocks"`
	WorldBookHandles     []WorldBookHandle   `json:"worldBookHandles"`
	Dependencies         []ProgramDependency `json:"dependencies"`
	ObservedCapabilities []string            `json:"observedCapabilities"`
	ReferencedVariables  []string            `json:"referencedVariables"`
	OmittedContent       []OmittedContent    `json:"omittedContent"`
	Redactions           []ProgramRedaction  `json:"redactions"`
}

type ProgramBlock struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	SourcePath     string `json:"sourcePath"`
	Name           string `json:"name"`
	Language       string `json:"language"`
	Content        string `json:"content"`
	OriginalSHA256 string `json:"originalSha256"`
	Enabled        bool   `json:"enabled"`
	TriggerPattern string `json:"triggerPattern,omitempty"`
	Placements     []int  `json:"placements"`
}

type WorldBookHandle struct {
	Handle        string `json:"handle"`
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	ContentChars  int    `json:"contentChars"`
	ContentSHA256 string `json:"contentSha256"`
}

type ProgramDependency struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Locator string `json:"locator"`
}

type OmittedContent struct {
	Field  string `json:"field"`
	Chars  int    `json:"chars"`
	SHA256 string `json:"sha256"`
}

type ProgramRedaction struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

type cardEnvelope struct {
	Data json.RawMessage `json:"data"`
}

type programCardData struct {
	Description             string                     `json:"description"`
	Personality             string                     `json:"personality"`
	Scenario                string                     `json:"scenario"`
	FirstMessage            string                     `json:"first_mes"`
	MessageExample          string                     `json:"mes_example"`
	SystemPrompt            string                     `json:"system_prompt"`
	PostHistoryInstructions string                     `json:"post_history_instructions"`
	CreatorNotes            string                     `json:"creator_notes"`
	CharacterBook           json.RawMessage            `json:"character_book"`
	Extensions              map[string]json.RawMessage `json:"extensions"`
}

type programRegex struct {
	ScriptName    string `json:"scriptName"`
	Name          string `json:"name"`
	FindRegex     string `json:"findRegex"`
	ReplaceString string `json:"replaceString"`
	Placement     []int  `json:"placement"`
	Disabled      bool   `json:"disabled"`
}

type scriptBlock struct {
	Path    string
	Name    string
	Content string
	Enabled bool
}

func ExtractProgramView(path, sourceSHA256 string) (ProgramView, error) {
	if !sha256Pattern.MatchString(sourceSHA256) {
		return ProgramView{}, errors.New("source SHA-256 is invalid")
	}
	raw, _, _, err := card.ReadDocumentFile(path)
	if err != nil {
		return ProgramView{}, fmt.Errorf("read program source: %w", err)
	}
	var envelope cardEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return ProgramView{}, fmt.Errorf("decode program source envelope: %w", err)
	}
	dataRaw := raw
	if len(strings.TrimSpace(string(envelope.Data))) > 0 && strings.TrimSpace(string(envelope.Data)) != "null" {
		dataRaw = envelope.Data
	}
	var data programCardData
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		return ProgramView{}, fmt.Errorf("decode program source data: %w", err)
	}

	sanitizer := newSanitizer()
	blocks := make([]ProgramBlock, 0)
	var regexes []programRegex
	if rawRegex, ok := data.Extensions["regex_scripts"]; ok {
		_ = json.Unmarshal(rawRegex, &regexes)
	}
	for index, item := range regexes {
		if !activeMarkup.MatchString(item.ReplaceString) {
			continue
		}
		blocks = append(blocks, ProgramBlock{
			ID:             fmt.Sprintf("markup-%d", len(blocks)+1),
			Kind:           "ACTIVE_MARKUP",
			SourcePath:     fmt.Sprintf("data.extensions.regex_scripts[%d].replaceString", index),
			Name:           sanitizer.sanitize(firstNonEmpty(item.ScriptName, item.Name)),
			Language:       "html",
			Content:        sanitizer.sanitize(item.ReplaceString),
			OriginalSHA256: textHash(item.ReplaceString),
			Enabled:        !item.Disabled,
			TriggerPattern: sanitizer.sanitize(item.FindRegex),
			Placements:     append([]int(nil), item.Placement...),
		})
	}
	for _, item := range collectScripts(data.Extensions) {
		blocks = append(blocks, ProgramBlock{
			ID:             fmt.Sprintf("script-%d", len(blocks)+1),
			Kind:           "SCRIPT",
			SourcePath:     item.Path,
			Name:           sanitizer.sanitize(item.Name),
			Language:       "javascript",
			Content:        sanitizer.sanitize(item.Content),
			OriginalSHA256: textHash(item.Content),
			Enabled:        item.Enabled,
			Placements:     []int{},
		})
	}

	programText := strings.Builder{}
	for _, block := range blocks {
		programText.WriteString(block.Content)
		programText.WriteByte('\n')
	}
	return ProgramView{
		SchemaVersion:        ProgramViewSchemaVersion,
		SourceSHA256:         sourceSHA256,
		ProgramBlocks:        blocks,
		WorldBookHandles:     worldBookHandles(data.CharacterBook, sanitizer),
		Dependencies:         sanitizer.dependencies,
		ObservedCapabilities: detectCapabilities(programText.String()),
		ReferencedVariables:  detectVariables(programText.String()),
		OmittedContent: omittedContent([]namedContent{
			{"description", data.Description},
			{"personality", data.Personality},
			{"scenario", data.Scenario},
			{"firstMessage", data.FirstMessage},
			{"messageExamples", data.MessageExample},
			{"systemPrompt", data.SystemPrompt},
			{"postHistoryInstructions", data.PostHistoryInstructions},
			{"creatorNotes", data.CreatorNotes},
		}),
		Redactions: sanitizer.redactions(),
	}, nil
}

func collectScripts(extensions map[string]json.RawMessage) []scriptBlock {
	keys := make([]string, 0, len(extensions))
	for key := range extensions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]scriptBlock, 0)
	var visit func(any, string)
	visit = func(value any, path string) {
		switch typed := value.(type) {
		case []any:
			for index, child := range typed {
				visit(child, fmt.Sprintf("%s[%d]", path, index))
			}
		case map[string]any:
			content, hasContent := typed["content"].(string)
			typeName, _ := typed["type"].(string)
			if hasContent && strings.TrimSpace(content) != "" &&
				(strings.Contains(strings.ToLower(path), "script") || strings.EqualFold(typeName, "script")) {
				name, _ := typed["name"].(string)
				enabled := true
				if explicit, ok := typed["enabled"].(bool); ok {
					enabled = explicit
				}
				result = append(result, scriptBlock{Path: path + ".content", Name: name, Content: content, Enabled: enabled})
			}
			childKeys := make([]string, 0, len(typed))
			for key := range typed {
				childKeys = append(childKeys, key)
			}
			sort.Strings(childKeys)
			for _, key := range childKeys {
				visit(typed[key], path+"."+key)
			}
		}
	}
	for _, key := range keys {
		var value any
		if json.Unmarshal(extensions[key], &value) == nil {
			visit(value, "data.extensions."+key)
		}
	}
	return result
}

func worldBookHandles(raw json.RawMessage, sanitizer *programSanitizer) []WorldBookHandle {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return []WorldBookHandle{}
	}
	var book map[string]any
	if json.Unmarshal(raw, &book) != nil {
		return []WorldBookHandle{}
	}
	entries, ok := book["entries"].([]any)
	if !ok {
		return []WorldBookHandle{}
	}
	result := make([]WorldBookHandle, 0, len(entries))
	for index, value := range entries {
		entry, ok := value.(map[string]any)
		if !ok {
			continue
		}
		id := scalarString(entry["id"])
		if id == "" {
			id = scalarString(entry["uid"])
		}
		if id == "" {
			id = strconv.Itoa(index)
		}
		name, _ := entry["name"].(string)
		if name == "" {
			name, _ = entry["comment"].(string)
		}
		content, _ := entry["content"].(string)
		enabled := true
		if value, ok := entry["enabled"].(bool); ok {
			enabled = value
		} else if disabled, ok := entry["disable"].(bool); ok {
			enabled = !disabled
		}
		result = append(result, WorldBookHandle{
			Handle:        fmt.Sprintf("worldbook:0:%s", id),
			Name:          sanitizer.sanitize(name),
			Enabled:       enabled,
			ContentChars:  len([]rune(content)),
			ContentSHA256: textHash(content),
		})
	}
	return result
}

type namedContent struct {
	name  string
	value string
}

func omittedContent(values []namedContent) []OmittedContent {
	result := make([]OmittedContent, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value.value) == "" {
			continue
		}
		result = append(result, OmittedContent{
			Field: value.name, Chars: len([]rune(value.value)), SHA256: textHash(value.value),
		})
	}
	return result
}

type programSanitizer struct {
	dependencyByRaw map[string]int
	dependencies    []ProgramDependency
	redactionOrder  []string
	redactionCounts map[string]int
}

func newSanitizer() *programSanitizer {
	return &programSanitizer{dependencyByRaw: map[string]int{}, redactionCounts: map[string]int{}}
}

func (s *programSanitizer) sanitize(source string) string {
	value := dataURI.ReplaceAllStringFunc(source, func(match string) string {
		s.count("inline-data")
		return fmt.Sprintf("data:<redacted>;bytes~%d", len(match))
	})
	value = remoteURL.ReplaceAllStringFunc(value, func(match string) string {
		raw := strings.TrimRight(match, ".,;")
		index, ok := s.dependencyByRaw[raw]
		if !ok {
			index = len(s.dependencies) + 1
			s.dependencyByRaw[raw] = index
			s.dependencies = append(s.dependencies, ProgramDependency{
				ID: fmt.Sprintf("dependency-%d", index), Kind: "remote-url", Locator: sanitizedURL(raw),
			})
		}
		return fmt.Sprintf("dependency://dependency-%d", index)
	})
	value = bearerSecret.ReplaceAllStringFunc(value, func(string) string {
		s.count("credential")
		return "Bearer <redacted-secret>"
	})
	value = secretAssignment.ReplaceAllStringFunc(value, func(match string) string {
		s.count("credential")
		separator := strings.IndexAny(match, ":=")
		if separator < 0 {
			return "<redacted-secret>"
		}
		return match[:separator+1] + " <redacted-secret>"
	})
	for _, pattern := range []*regexp.Regexp{windowsUserPath, unixUserPath} {
		value = pattern.ReplaceAllStringFunc(value, func(string) string {
			s.count("local-path")
			return "<redacted-local-path>"
		})
	}
	return value
}

func (s *programSanitizer) count(kind string) {
	if s.redactionCounts[kind] == 0 {
		s.redactionOrder = append(s.redactionOrder, kind)
	}
	s.redactionCounts[kind]++
}

func (s *programSanitizer) redactions() []ProgramRedaction {
	result := make([]ProgramRedaction, 0, len(s.redactionOrder))
	for _, kind := range s.redactionOrder {
		result = append(result, ProgramRedaction{Kind: kind, Count: s.redactionCounts[kind]})
	}
	return result
}

func sanitizedURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return "<redacted-url>"
	}
	host := parsed.Hostname()
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if parsed.Port() != "" {
		host += ":" + parsed.Port()
	}
	return parsed.Scheme + "://" + host + parsed.EscapedPath()
}

func detectCapabilities(text string) []string {
	result := make([]string, 0)
	for _, capability := range capabilities {
		for _, pattern := range capability.patterns {
			if pattern.MatchString(text) {
				result = append(result, capability.name)
				break
			}
		}
	}
	sort.Strings(result)
	return result
}

func detectVariables(text string) []string {
	values := map[string]struct{}{}
	for _, match := range functionVariable.FindAllStringSubmatch(text, -1) {
		if len(match) > 2 && strings.TrimSpace(match[2]) != "" {
			values[match[2]] = struct{}{}
		}
	}
	for _, match := range macroVariable.FindAllStringSubmatch(text, -1) {
		if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			values[strings.TrimSpace(match[1])] = struct{}{}
		}
	}
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func scalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func textHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

type capabilityPatterns struct {
	name     string
	patterns []*regexp.Regexp
}

var (
	sha256Pattern    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	activeMarkup     = regexp.MustCompile(`(?i)<(?:html|style|script|form|button|input|select|textarea)\b`)
	remoteURL        = regexp.MustCompile("(?i)https?://[^\\s'\"`<>\\)]+")
	dataURI          = regexp.MustCompile("(?i)data:[^\\s'\"`<>]+")
	bearerSecret     = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]{8,}`)
	secretAssignment = regexp.MustCompile("(?i)[\"']?(?:api[_-]?key|access[_-]?token|auth[_-]?token|password|secret)[\"']?\\s*[:=]\\s*[\"']?[^'\"`\\s,;}]{6,}")
	windowsUserPath  = regexp.MustCompile("(?i)\\b[A-Z]:\\\\+(?:Users|Documents and Settings)\\\\+[^\\s'\"`<>]+")
	unixUserPath     = regexp.MustCompile("/(?:home|Users)/[^\\s'\"`<>]+")
	functionVariable = regexp.MustCompile(`(?i)\b(getvar|setvar|addvar|incvar|decvar|getglobalvar|setglobalvar|get_message_variable|set_message_variable)\s*\(\s*['"]([^'"]+)['"]`)
	macroVariable    = regexp.MustCompile(`(?i)\{\{\s*(?:getvar|getglobalvar|get_message_variable)::([^}]+)}}`)
	capabilities     = []capabilityPatterns{
		{"chat.read", []*regexp.Regexp{regexp.MustCompile(`\bgetChatMessages\b`), regexp.MustCompile(`\bgetCurrentMessageId\b`)}},
		{"chat.write", []*regexp.Regexp{regexp.MustCompile(`\bsetChatMessage\b`), regexp.MustCompile(`#send_textarea`), regexp.MustCompile(`\bcreateChatMessages\b`)}},
		{"event.subscribe", []*regexp.Regexp{regexp.MustCompile(`\beventOn\b`), regexp.MustCompile(`addEventListener\s*\(`)}},
		{"event.emit", []*regexp.Regexp{regexp.MustCompile(`\beventEmit\b`)}},
		{"generation.request", []*regexp.Regexp{regexp.MustCompile(`\bgenerate\s*\(`)}},
		{"state.read", []*regexp.Regexp{regexp.MustCompile(`(?i)\b(?:getvar|getglobalvar|get_message_variable)\b`)}},
		{"state.write", []*regexp.Regexp{regexp.MustCompile(`(?i)\b(?:setvar|addvar|incvar|decvar|setglobalvar|set_message_variable)\b`)}},
		{"slash.execute", []*regexp.Regexp{regexp.MustCompile(`\btriggerSlash\b`)}},
		{"worldbook.control", []*regexp.Regexp{regexp.MustCompile(`(?i)\b(?:worldbook|lorebook)\b`)}},
	}
)
