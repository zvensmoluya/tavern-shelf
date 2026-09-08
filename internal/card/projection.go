package card

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

func readCardBytes(r io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxSourceSize+1))
	if err != nil {
		return nil, fmt.Errorf("read character card: %w", err)
	}
	if len(raw) > maxCardSize {
		return nil, fmt.Errorf("character card exceeds the %d MiB source size limit", maxCardSize>>20)
	}
	return raw, nil
}

// A readable top-level spec identifies a card even when later JSON is damaged.
// Substrings inside descriptions, scripts or nested objects are not evidence.
func hasReadableCardSpec(raw []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return false
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return false
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return false
		}
		if key == "spec" {
			var spec string
			if json.Unmarshal(value, &spec) == nil && isCharacterEnvelope(envelope{Spec: spec}) {
				return true
			}
		}
	}
	return false
}

// Project known fields independently. A display field cannot veto storage of
// an identified card, and a malformed array entry cannot discard its siblings.
// Raw extension data stays opaque; no card content is evaluated or rewritten.
func projectJSON(raw []byte, destination any, path string, warnings *[]string) {
	projectValue(raw, reflect.ValueOf(destination).Elem(), path, warnings)
}

var rawMessageType = reflect.TypeFor[json.RawMessage]()

func projectionWarning(path string, warnings *[]string) {
	if len(*warnings) < 64 {
		*warnings = append(*warnings, "字段 "+path+" 未能按原格式展示；原始值已保留。")
	}
}

func projectValue(raw []byte, value reflect.Value, path string, warnings *[]string) bool {
	raw = bytes.TrimSpace(raw)
	if value.Type() == rawMessageType {
		value.SetBytes(append([]byte(nil), raw...))
		return true
	}
	if bytes.Equal(raw, []byte("null")) {
		return false
	}
	if value.Kind() == reflect.Pointer {
		target := reflect.New(value.Type().Elem())
		if !projectValue(raw, target.Elem(), path, warnings) {
			return false
		}
		value.Set(target)
		return true
	}
	switch value.Kind() {
	case reflect.Struct:
		var object map[string]json.RawMessage
		if json.Unmarshal(raw, &object) == nil && object != nil {
			for index := 0; index < value.NumField(); index++ {
				field := value.Type().Field(index)
				key := strings.Split(field.Tag.Get("json"), ",")[0]
				if item, ok := object[key]; ok && value.Field(index).CanSet() {
					fieldPath := key
					if path != "" {
						fieldPath = path + "." + key
					}
					projectValue(item, value.Field(index), fieldPath, warnings)
				}
			}
			return true
		}
	case reflect.Map:
		var object map[string]json.RawMessage
		if json.Unmarshal(raw, &object) == nil && object != nil {
			value.Set(reflect.MakeMap(value.Type()))
			keys := make([]string, 0, len(object))
			for key := range object {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				item := reflect.New(value.Type().Elem()).Elem()
				if projectValue(object[key], item, path+"."+key, warnings) {
					value.SetMapIndex(reflect.ValueOf(key), item)
				}
			}
			return true
		}
	case reflect.Slice:
		var items []json.RawMessage
		if json.Unmarshal(raw, &items) != nil {
			// A scalar string list has an unambiguous display projection.
			if value.Type().Elem().Kind() != reflect.String || len(raw) == 0 || raw[0] != '"' {
				break
			}
			items = []json.RawMessage{raw}
			projectionWarning(path, warnings)
		}
		value.Set(reflect.MakeSlice(value.Type(), 0, len(items)))
		for index, item := range items {
			target := reflect.New(value.Type().Elem()).Elem()
			if projectValue(item, target, fmt.Sprintf("%s[%d]", path, index), warnings) {
				value.Set(reflect.Append(value, target))
			}
		}
		return true
	case reflect.String:
		var text string
		if json.Unmarshal(raw, &text) == nil {
			value.SetString(text)
			return true
		}
		if len(raw) > 0 && (raw[0] == '-' || raw[0] >= '0' && raw[0] <= '9') && json.Valid(raw) {
			value.SetString(string(raw))
			projectionWarning(path, warnings)
			return true
		}
	case reflect.Bool, reflect.Int, reflect.Int64:
		if json.Unmarshal(raw, value.Addr().Interface()) == nil {
			return true
		}
		var text string
		if json.Unmarshal(raw, &text) == nil {
			if value.Kind() == reflect.Bool {
				if text == "true" || text == "false" {
					value.SetBool(text == "true")
					projectionWarning(path, warnings)
					return true
				}
			} else if number, err := strconv.ParseInt(text, 10, value.Type().Bits()); err == nil {
				value.SetInt(number)
				projectionWarning(path, warnings)
				return true
			}
		}
	}
	projectionWarning(path, warnings)
	return false
}
