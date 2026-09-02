package adaptation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	ArtifactSchemaVersion = 1
	MaxArtifactSize       = 2 << 20
)

type Artifact struct {
	SchemaVersion        int               `json:"schemaVersion"`
	SourceSHA256         string            `json:"sourceSha256"`
	Compiler             ArtifactCompiler  `json:"compiler"`
	Status               string            `json:"status"`
	RequiredCapabilities []string          `json:"requiredCapabilities"`
	State                []StateDefinition `json:"state"`
	Views                []View            `json:"views"`
	Report               ArtifactReport    `json:"report"`
}

type ArtifactCompiler struct {
	ID      string  `json:"id"`
	Version string  `json:"version"`
	Model   *string `json:"model,omitempty"`
}

type StateDefinition struct {
	Key          string          `json:"key"`
	Type         string          `json:"type"`
	InitialValue json.RawMessage `json:"initialValue"`
}

type View struct {
	ID            string      `json:"id"`
	Title         string      `json:"title"`
	Placement     string      `json:"placement"`
	Trigger       ViewTrigger `json:"trigger"`
	Nodes         []UINode    `json:"nodes"`
	SubmitLabel   *string     `json:"submitLabel,omitempty"`
	SubmitActions []Action    `json:"submitActions"`
}

type ViewTrigger struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type UINode struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Title    string      `json:"title"`
	Text     string      `json:"text"`
	StateKey *string     `json:"stateKey,omitempty"`
	Min      *float64    `json:"min,omitempty"`
	Max      *float64    `json:"max,omitempty"`
	Children []UINode    `json:"children"`
	Fields   []FormField `json:"fields"`
}

type FormField struct {
	ID           string       `json:"id"`
	Type         string       `json:"type"`
	Label        string       `json:"label"`
	Placeholder  string       `json:"placeholder"`
	Required     bool         `json:"required"`
	Options      []FormOption `json:"options"`
	InitialValue string       `json:"initialValue"`
}

type FormOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Action struct {
	Type     string  `json:"type"`
	Target   *string `json:"target,omitempty"`
	Value    *string `json:"value,omitempty"`
	Template *string `json:"template,omitempty"`
}

type ArtifactReport struct {
	Summary              string   `json:"summary"`
	RestoredBehaviors    []string `json:"restoredBehaviors"`
	UnsupportedBehaviors []string `json:"unsupportedBehaviors"`
	Warnings             []string `json:"warnings"`
}

type ValidationIssue struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func DecodeArtifact(source io.Reader) (Artifact, []byte, error) {
	raw, err := io.ReadAll(io.LimitReader(source, MaxArtifactSize+1))
	if err != nil {
		return Artifact{}, nil, fmt.Errorf("read adaptation artifact: %w", err)
	}
	if len(raw) > MaxArtifactSize {
		return Artifact{}, nil, errors.New("adaptation artifact exceeds size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var artifact Artifact
	if err := decoder.Decode(&artifact); err != nil {
		return Artifact{}, nil, fmt.Errorf("decode adaptation artifact: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return Artifact{}, nil, errors.New("decode adaptation artifact: trailing JSON value")
	}
	return artifact, raw, nil
}

func ValidateArtifact(artifact Artifact, expectedSourceSHA256 string) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	issue := func(path, code, message string) {
		issues = append(issues, ValidationIssue{Path: path, Code: code, Message: message})
	}
	if artifact.SchemaVersion != ArtifactSchemaVersion {
		issue("schemaVersion", "UNSUPPORTED_SCHEMA", "仅支持 adaptation schema v1")
	}
	if !sha256Pattern.MatchString(artifact.SourceSHA256) {
		issue("sourceSha256", "INVALID_SOURCE_HASH", "sourceSha256 必须是小写 SHA-256")
	}
	if expectedSourceSHA256 != "" && artifact.SourceSHA256 != expectedSourceSHA256 {
		issue("sourceSha256", "SOURCE_HASH_MISMATCH", "适配产物与原始角色卡不匹配")
	}
	validateText("compiler.id", artifact.Compiler.ID, maxIDChars, &issues)
	validateText("compiler.version", artifact.Compiler.Version, maxIDChars, &issues)
	if strings.TrimSpace(artifact.Compiler.ID) == "" {
		issue("compiler.id", "EMPTY_VALUE", "compiler id 不能为空")
	}
	if strings.TrimSpace(artifact.Compiler.Version) == "" {
		issue("compiler.version", "EMPTY_VALUE", "compiler version 不能为空")
	}
	if artifact.Status != "FULL" && artifact.Status != "PARTIAL" {
		issue("status", "UNKNOWN_ENUM", "status 必须是 FULL 或 PARTIAL")
	}
	if len(artifact.Views) > maxViews {
		issue("views", "TOO_MANY_VIEWS", "视图数量超过限制")
	}
	if len(artifact.State) > maxStateValues {
		issue("state", "TOO_MANY_STATE_VALUES", "状态数量超过限制")
	}

	stateKeys := map[string]string{}
	for index, definition := range artifact.State {
		path := fmt.Sprintf("state[%d]", index)
		validateID(path+".key", definition.Key, &issues)
		if _, exists := stateKeys[definition.Key]; exists {
			issue(path+".key", "DUPLICATE_ID", "状态 key 重复")
		}
		stateKeys[definition.Key] = definition.Type
		if !validInitialValue(definition.Type, definition.InitialValue) {
			issue(path+".initialValue", "STATE_TYPE_MISMATCH", "初始值与状态类型不匹配")
		}
	}

	viewIDs := map[string]bool{}
	totalNodes := 0
	for viewIndex, view := range artifact.Views {
		path := fmt.Sprintf("views[%d]", viewIndex)
		validateID(path+".id", view.ID, &issues)
		if viewIDs[view.ID] {
			issue(path+".id", "DUPLICATE_ID", "视图 id 重复")
		}
		viewIDs[view.ID] = true
		validateText(path+".title", view.Title, maxTextChars, &issues)
		if !oneOf(view.Placement, "MESSAGE_REPLACEMENT", "MESSAGE_ATTACHMENT", "CONVERSATION_HEADER") {
			issue(path+".placement", "UNKNOWN_ENUM", "未知的视图位置")
		}
		switch view.Trigger.Type {
		case "ALWAYS":
			if view.Trigger.Value != "" {
				issue(path+".trigger.value", "UNUSED_VALUE", "ALWAYS trigger 不接受 value")
			}
		case "MESSAGE_EXACT", "MESSAGE_CONTAINS":
			if strings.TrimSpace(view.Trigger.Value) == "" {
				issue(path+".trigger.value", "EMPTY_TRIGGER", "消息 trigger 不能为空")
			}
			validateText(path+".trigger.value", view.Trigger.Value, maxTriggerChars, &issues)
		default:
			issue(path+".trigger.type", "UNKNOWN_ENUM", "未知的 trigger 类型")
		}

		fieldIDs := map[string]bool{}
		nodeIDs := map[string]bool{}
		var validateNode func(UINode, string, int)
		validateNode = func(node UINode, nodePath string, depth int) {
			totalNodes++
			if depth > maxNodeDepth {
				issue(nodePath, "UI_TOO_DEEP", "UI 嵌套深度超过限制")
			}
			validateID(nodePath+".id", node.ID, &issues)
			if nodeIDs[node.ID] {
				issue(nodePath+".id", "DUPLICATE_ID", "同一视图中的 UI 节点 id 重复")
			}
			nodeIDs[node.ID] = true
			validateText(nodePath+".title", node.Title, maxTextChars, &issues)
			validateText(nodePath+".text", node.Text, maxTextChars, &issues)
			if !oneOf(node.Type, "SECTION", "TEXT", "STATUS", "FORM") {
				issue(nodePath+".type", "UNKNOWN_ENUM", "未知的 UI 节点类型")
			}
			if node.Type == "STATUS" {
				if node.StateKey == nil || stateKeys[*node.StateKey] == "" {
					issue(nodePath+".stateKey", "UNKNOWN_STATE", "状态组件引用了未知状态")
				}
				if node.Min != nil && node.Max != nil && *node.Min >= *node.Max {
					issue(nodePath, "INVALID_RANGE", "状态组件的 min 必须小于 max")
				}
			} else if node.StateKey != nil {
				issue(nodePath+".stateKey", "UNUSED_VALUE", "只有 STATUS 组件可以绑定 stateKey")
			}
			if node.Type != "FORM" && len(node.Fields) > 0 {
				issue(nodePath+".fields", "UNEXPECTED_FIELDS", "只有 FORM 组件可以包含 fields")
			}
			if len(node.Fields) > maxFieldsPerForm {
				issue(nodePath+".fields", "TOO_MANY_FIELDS", "单个表单字段数量超过限制")
			}
			for fieldIndex, field := range node.Fields {
				fieldPath := fmt.Sprintf("%s.fields[%d]", nodePath, fieldIndex)
				validateID(fieldPath+".id", field.ID, &issues)
				if fieldIDs[field.ID] {
					issue(fieldPath+".id", "DUPLICATE_ID", "同一视图中的表单字段 id 重复")
				}
				fieldIDs[field.ID] = true
				validateText(fieldPath+".label", field.Label, maxTextChars, &issues)
				validateText(fieldPath+".placeholder", field.Placeholder, maxTextChars, &issues)
				validateText(fieldPath+".initialValue", field.InitialValue, maxInputChars, &issues)
				needsOptions := field.Type == "SINGLE_SELECT" || field.Type == "MULTI_SELECT"
				if !oneOf(field.Type, "TEXT", "MULTILINE_TEXT", "NUMBER", "SINGLE_SELECT", "MULTI_SELECT", "TOGGLE") {
					issue(fieldPath+".type", "UNKNOWN_ENUM", "未知的表单字段类型")
				}
				if needsOptions && len(field.Options) == 0 {
					issue(fieldPath+".options", "MISSING_OPTIONS", "选择字段必须包含选项")
				}
				if !needsOptions && len(field.Options) > 0 {
					issue(fieldPath+".options", "UNEXPECTED_OPTIONS", "该字段类型不接受选项")
				}
				if len(field.Options) > maxOptions {
					issue(fieldPath+".options", "TOO_MANY_OPTIONS", "选项数量超过限制")
				}
				for optionIndex, option := range field.Options {
					validateText(fmt.Sprintf("%s.options[%d].value", fieldPath, optionIndex), option.Value, maxTextChars, &issues)
					validateText(fmt.Sprintf("%s.options[%d].label", fieldPath, optionIndex), option.Label, maxTextChars, &issues)
				}
			}
			for childIndex, child := range node.Children {
				validateNode(child, fmt.Sprintf("%s.children[%d]", nodePath, childIndex), depth+1)
			}
		}
		for nodeIndex, node := range view.Nodes {
			validateNode(node, fmt.Sprintf("%s.nodes[%d]", path, nodeIndex), 1)
		}
		if len(view.SubmitActions) > maxActions {
			issue(path+".submitActions", "TOO_MANY_ACTIONS", "动作数量超过限制")
		}
		for actionIndex, action := range view.SubmitActions {
			actionPath := fmt.Sprintf("%s.submitActions[%d]", path, actionIndex)
			switch action.Type {
			case "CHAT_SET_DRAFT":
				if action.Template == nil || strings.TrimSpace(*action.Template) == "" {
					issue(actionPath+".template", "MISSING_TEMPLATE", "写入草稿需要 template")
				}
				if action.Target != nil || action.Value != nil {
					issue(actionPath, "UNUSED_VALUE", "写入草稿不接受 target/value")
				}
			case "STATE_SET", "STATE_INCREMENT":
				if action.Target == nil || stateKeys[*action.Target] == "" {
					issue(actionPath+".target", "UNKNOWN_STATE", "动作引用了未知状态")
				}
				if action.Value == nil && action.Template == nil {
					issue(actionPath, "MISSING_VALUE", "状态动作需要 value 或 template")
				}
			case "STATE_TOGGLE":
				if action.Target == nil || stateKeys[*action.Target] != "BOOLEAN" {
					issue(actionPath+".target", "STATE_TYPE_MISMATCH", "toggle 只能用于布尔状态")
				}
				if action.Value != nil || action.Template != nil {
					issue(actionPath, "UNUSED_VALUE", "toggle 不接受 value/template")
				}
			default:
				issue(actionPath+".type", "UNKNOWN_ENUM", "未知的动作类型")
			}
			if action.Template != nil {
				validateText(actionPath+".template", *action.Template, maxTemplateChars, &issues)
				validateTemplate(actionPath+".template", *action.Template, fieldIDs, stateKeys, &issues)
			}
		}
	}
	if totalNodes > maxNodes {
		issue("views", "TOO_MANY_NODES", "UI 节点总数超过限制")
	}

	inferred := map[string]bool{}
	for _, view := range artifact.Views {
		inferred["ui.native"] = true
		for _, action := range view.SubmitActions {
			if action.Type == "CHAT_SET_DRAFT" {
				inferred["chat.setDraft"] = true
			} else if strings.HasPrefix(action.Type, "STATE_") {
				inferred["state.write"] = true
			}
		}
	}
	declared := map[string]bool{}
	for index, capability := range artifact.RequiredCapabilities {
		declared[capability] = true
		if !oneOf(capability, "ui.native", "chat.setDraft", "state.write") {
			issue(fmt.Sprintf("requiredCapabilities[%d]", index), "UNSUPPORTED_CAPABILITY", "Player 不支持该 capability")
		}
	}
	for capability := range inferred {
		if !declared[capability] {
			issue("requiredCapabilities", "MISSING_CAPABILITY", "产物未声明实际使用的 capability: "+capability)
		}
	}
	return deduplicateIssues(issues)
}

func validInitialValue(kind string, raw json.RawMessage) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if len(raw) == 0 || decoder.Decode(&value) != nil {
		return false
	}
	switch kind {
	case "STRING":
		_, ok := value.(string)
		return ok
	case "NUMBER":
		_, ok := value.(json.Number)
		return ok
	case "BOOLEAN":
		_, ok := value.(bool)
		return ok
	default:
		return false
	}
}

func validateID(path, value string, issues *[]ValidationIssue) {
	if !idPattern.MatchString(value) {
		*issues = append(*issues, ValidationIssue{Path: path, Code: "INVALID_ID", Message: "id 格式无效"})
	}
}

func validateText(path, value string, maxChars int, issues *[]ValidationIssue) {
	if utf8.RuneCountInString(value) > maxChars {
		*issues = append(*issues, ValidationIssue{Path: path, Code: "TEXT_TOO_LONG", Message: "文本超过长度限制"})
	}
	if externalIO.MatchString(value) {
		*issues = append(*issues, ValidationIssue{Path: path, Code: "EXTERNAL_IO_FORBIDDEN", Message: "适配产物不能包含外部 URL 或文件 URI"})
	}
}

func validateTemplate(path, value string, fields map[string]bool, states map[string]string, issues *[]ValidationIssue) {
	for _, match := range anyTemplateReference.FindAllStringSubmatch(value, -1) {
		reference := strings.TrimSpace(match[1])
		if reference == "user" || reference == "char" {
			continue
		}
		parts := strings.Split(reference, ".")
		valid := len(parts) == 2 && idPattern.MatchString(parts[1]) &&
			((parts[0] == "form" && fields[parts[1]]) || (parts[0] == "state" && states[parts[1]] != ""))
		if !valid {
			*issues = append(*issues, ValidationIssue{Path: path, Code: "UNKNOWN_TEMPLATE_REFERENCE", Message: "模板引用了未知值 " + reference})
		}
	}
}

func deduplicateIssues(source []ValidationIssue) []ValidationIssue {
	result := make([]ValidationIssue, 0, len(source))
	seen := map[ValidationIssue]bool{}
	for _, item := range source {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

func oneOf(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}

var (
	idPattern            = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+){0,15}$`)
	externalIO           = regexp.MustCompile(`(?i)(?:https?://|data:|file:|content:)`)
	anyTemplateReference = regexp.MustCompile(`\{\{([^{}]+)}}`)
)

const (
	maxViews         = 16
	maxStateValues   = 128
	maxNodes         = 256
	maxNodeDepth     = 8
	maxFieldsPerForm = 32
	maxOptions       = 64
	maxActions       = 16
	maxIDChars       = 80
	maxTextChars     = 4_096
	maxInputChars    = 8_192
	maxTriggerChars  = 256
	maxTemplateChars = 16_384
)
