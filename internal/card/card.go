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
	"strings"
)

var (
	ErrUnsupported = errors.New("unsupported character card format")
	ErrInvalidPNG  = errors.New("invalid PNG character card")
)

const maxCardSize = 64 << 20

type Character struct {
	Name           string   `json:"name"`
	Creator        string   `json:"creator,omitempty"`
	Spec           string   `json:"spec,omitempty"`
	SpecVersion    string   `json:"specVersion,omitempty"`
	Tags           []string `json:"tags"`
	HasWorldbook   bool     `json:"hasWorldbook"`
	HasRegex       bool     `json:"hasRegex"`
	HasExtensions  bool     `json:"hasExtensions"`
	HasInteractive bool     `json:"hasInteractive"`
	SourceFormat   string   `json:"sourceFormat"`
	SourceIsImage  bool     `json:"sourceIsImage"`
}

type envelope struct {
	Spec        string          `json:"spec"`
	SpecVersion string          `json:"spec_version"`
	Name        string          `json:"name"`
	Creator     string          `json:"creator"`
	Tags        []string        `json:"tags"`
	Data        json.RawMessage `json:"data"`
}

type cardData struct {
	Name          string                     `json:"name"`
	Creator       string                     `json:"creator"`
	Tags          []string                   `json:"tags"`
	CharacterBook json.RawMessage            `json:"character_book"`
	Extensions    map[string]json.RawMessage `json:"extensions"`
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
	if len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return Character{}, fmt.Errorf("decode character card data: %w", err)
		}
	}
	name := firstNonEmpty(data.Name, env.Name)
	if strings.TrimSpace(name) == "" {
		return Character{}, errors.New("character card has no name")
	}
	tags := data.Tags
	if len(tags) == 0 {
		tags = env.Tags
	}
	extensions := data.Extensions
	return Character{
		Name:           strings.TrimSpace(name),
		Creator:        strings.TrimSpace(firstNonEmpty(data.Creator, env.Creator)),
		Spec:           strings.TrimSpace(env.Spec),
		SpecVersion:    strings.TrimSpace(env.SpecVersion),
		Tags:           cleanTags(tags),
		HasWorldbook:   present(data.CharacterBook),
		HasRegex:       extensionMatches(extensions, "regex"),
		HasExtensions:  len(extensions) > 0,
		HasInteractive: extensionMatches(extensions, "interactive", "risu", "depth_prompt", "tavern_helper"),
		SourceFormat:   format,
		SourceIsImage:  image,
	}, nil
}

func ParsePNG(r io.Reader) (Character, error) {
	signature := make([]byte, 8)
	if _, err := io.ReadFull(r, signature); err != nil || !bytes.Equal(signature, []byte("\x89PNG\r\n\x1a\n")) {
		return Character{}, ErrInvalidPNG
	}
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
		if keyword == "chara" || keyword == "ccv3" {
			decoded, err := decodePayload(payload, keyword == "chara")
			if err != nil {
				return Character{}, err
			}
			return parseJSONBytes(decoded, "png", true)
		}
		if string(kind) == "IEND" {
			return Character{}, fmt.Errorf("%w: character metadata chunk not found", ErrInvalidPNG)
		}
	}
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

func decodePayload(payload []byte, preferBase64 bool) ([]byte, error) {
	trimmed := bytes.TrimSpace(payload)
	if !preferBase64 && json.Valid(trimmed) {
		return trimmed, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(string(trimmed))
	if err != nil {
		if json.Valid(trimmed) {
			return trimmed, nil
		}
		return nil, fmt.Errorf("decode PNG character metadata: %w", err)
	}
	return decoded, nil
}

func present(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) && !bytes.Equal(trimmed, []byte("{}"))
}

func extensionMatches(values map[string]json.RawMessage, needles ...string) bool {
	for key, raw := range values {
		lower := strings.ToLower(key + " " + string(raw))
		for _, needle := range needles {
			if strings.Contains(lower, needle) {
				return true
			}
		}
	}
	return false
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
