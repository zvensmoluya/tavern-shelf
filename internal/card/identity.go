package card

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
)

// Fingerprint the complete payload, including unknown extensions and envelope
// fields. Never fingerprint the lossy display manifest. Numbers retain precision.
// Duplicate object keys are ambiguous, so those payloads remain unclassified.
func contentFingerprint(raw []byte) string {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, ok := canonicalValue(decoder, 0)
	if !ok {
		return ""
	}
	if _, err := decoder.Token(); err != io.EOF {
		return ""
	}
	if _, ok := value.(map[string]any); !ok {
		return ""
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func canonicalValue(d *json.Decoder, depth int) (any, bool) {
	if depth > 256 {
		return nil, false
	}
	token, err := d.Token()
	if err != nil {
		return nil, false
	}
	delim, container := token.(json.Delim)
	if !container {
		return token, true
	}
	switch delim {
	case '{':
		result := map[string]any{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return nil, false
			}
			name, ok := key.(string)
			if !ok {
				return nil, false
			}
			if _, exists := result[name]; exists {
				return nil, false
			}
			value, ok := canonicalValue(d, depth+1)
			if !ok {
				return nil, false
			}
			result[name] = value
		}
		end, err := d.Token()
		return result, err == nil && end == json.Delim('}')
	case '[':
		result := []any{}
		for d.More() {
			value, ok := canonicalValue(d, depth+1)
			if !ok {
				return nil, false
			}
			result = append(result, value)
		}
		end, err := d.Token()
		return result, err == nil && end == json.Delim(']')
	}
	return nil, false
}

func identityKey(name, creator string, missing bool) string {
	name, creator = strings.TrimSpace(name), strings.TrimSpace(creator)
	if missing || name == "" || creator == "" {
		return ""
	}
	encoded, _ := json.Marshal([]string{name, creator})
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}
