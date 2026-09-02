package adaptation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	CompilerID      = "tavern-shelf-ai"
	CompilerVersion = "2"
	maxAttempts     = 3
	maxRepairIssues = 20
)

type CompilerConfig struct {
	BaseURL         string
	APIKey          string
	Model           string
	MaxOutputTokens int
	HTTPClient      *http.Client
}

type Compiler struct {
	endpoint        string
	apiKey          string
	model           string
	maxOutputTokens int
	client          *http.Client
}

type CompileResult struct {
	Artifact     Artifact
	Raw          []byte
	InputTokens  int64
	OutputTokens int64
	Attempts     int
}

func NewCompiler(config CompilerConfig) (*Compiler, error) {
	baseURL, err := url.Parse(strings.TrimSpace(config.BaseURL))
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, errors.New("compiler Base URL must be an absolute HTTP(S) URL")
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, errors.New("compiler Base URL must use HTTP(S)")
	}
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, errors.New("compiler API key is required")
	}
	if strings.TrimSpace(config.Model) == "" {
		return nil, errors.New("compiler model is required")
	}
	baseURL.RawQuery = ""
	baseURL.Fragment = ""
	baseURL.Path = strings.TrimRight(baseURL.Path, "/")
	if !strings.HasSuffix(baseURL.Path, "/responses") {
		baseURL.Path += "/responses"
	}
	if config.MaxOutputTokens <= 0 {
		config.MaxOutputTokens = 16_384
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 5 * time.Minute}
	}
	return &Compiler{
		endpoint: baseURL.String(), apiKey: config.APIKey, model: config.Model,
		maxOutputTokens: config.MaxOutputTokens, client: config.HTTPClient,
	}, nil
}

func (c *Compiler) Compile(ctx context.Context, view ProgramView) (CompileResult, error) {
	if view.SchemaVersion != ProgramViewSchemaVersion || !sha256Pattern.MatchString(view.SourceSHA256) {
		return CompileResult{}, errors.New("cannot compile an invalid Program View identity")
	}
	viewJSON, err := json.Marshal(view)
	if err != nil {
		return CompileResult{}, fmt.Errorf("encode Program View: %w", err)
	}
	prompt := compilePrompt(string(viewJSON))
	var totalInput, totalOutput int64
	var previous string
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		currentPrompt := prompt
		if attempt > 1 {
			currentPrompt = repairPrompt(prompt, previous, lastErr)
		}
		output, usage, err := c.request(ctx, currentPrompt)
		totalInput += usage.InputTokens
		totalOutput += usage.OutputTokens
		if err != nil {
			return CompileResult{}, err
		}
		previous = output
		artifactJSON, err := extractJSONObject(output)
		if err != nil {
			lastErr = err
			continue
		}
		artifact, _, err := DecodeArtifact(strings.NewReader(artifactJSON))
		if err != nil {
			lastErr = err
			continue
		}
		artifact.Compiler = ArtifactCompiler{ID: CompilerID, Version: CompilerVersion, Model: &c.model}
		if issues := ValidateArtifactForProgramView(artifact, view); len(issues) != 0 {
			lastErr = validationProblems(issues)
			continue
		}
		normalizeArtifact(&artifact)
		raw, err := json.MarshalIndent(artifact, "", "  ")
		if err != nil {
			return CompileResult{}, fmt.Errorf("encode validated adaptation artifact: %w", err)
		}
		raw = append(raw, '\n')
		return CompileResult{
			Artifact: artifact, Raw: raw, InputTokens: totalInput,
			OutputTokens: totalOutput, Attempts: attempt,
		}, nil
	}
	return CompileResult{}, fmt.Errorf("model did not produce a valid adaptation artifact after repair: %w", lastErr)
}

func validationProblems(issues []ValidationIssue) error {
	limit := min(len(issues), maxRepairIssues)
	problems := make([]string, 0, limit)
	for _, issue := range issues[:limit] {
		problems = append(problems, fmt.Sprintf("%s at %s", issue.Code, issue.Path))
	}
	if len(issues) > limit {
		problems = append(problems, fmt.Sprintf("and %d more", len(issues)-limit))
	}
	return errors.New(strings.Join(problems, "; "))
}

type responsesRequest struct {
	Model           string             `json:"model"`
	Input           []responsesMessage `json:"input"`
	MaxOutputTokens int                `json:"max_output_tokens"`
	Reasoning       responsesReasoning `json:"reasoning"`
	Text            responsesText      `json:"text"`
	Store           bool               `json:"store"`
}

type responsesReasoning struct {
	Effort string `json:"effort"`
}

type responsesText struct {
	Format responsesTextFormat `json:"format"`
}

type responsesTextFormat struct {
	Type string `json:"type"`
}

type responsesMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responsesUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

type responsesResponse struct {
	Status            string          `json:"status"`
	OutputText        string          `json:"output_text"`
	Output            []responsesItem `json:"output"`
	Usage             responsesUsage  `json:"usage"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type responsesItem struct {
	Type    string             `json:"type"`
	Content []responsesContent `json:"content"`
}

type responsesContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (c *Compiler) request(ctx context.Context, prompt string) (string, responsesUsage, error) {
	body, err := json.Marshal(responsesRequest{
		Model: c.model,
		Input: []responsesMessage{
			{Role: "developer", Content: compilerInstruction},
			{Role: "user", Content: prompt},
		},
		MaxOutputTokens: c.maxOutputTokens,
		Reasoning:       responsesReasoning{Effort: "none"},
		Text:            responsesText{Format: responsesTextFormat{Type: "json_object"}},
		Store:           false,
	})
	if err != nil {
		return "", responsesUsage{}, fmt.Errorf("encode model request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", responsesUsage{}, fmt.Errorf("create model request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return "", responsesUsage{}, fmt.Errorf("request adaptation model: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return "", responsesUsage{}, fmt.Errorf("adaptation model returned HTTP %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 8<<20))
	var decoded responsesResponse
	if err := decoder.Decode(&decoded); err != nil {
		return "", responsesUsage{}, fmt.Errorf("decode adaptation model response: %w", err)
	}
	if decoded.Error != nil {
		return "", decoded.Usage, errors.New("adaptation model reported an error")
	}
	text := decoded.OutputText
	if text == "" {
		var builder strings.Builder
		for _, item := range decoded.Output {
			if item.Type != "message" {
				continue
			}
			for _, content := range item.Content {
				if content.Type == "output_text" || content.Type == "text" {
					builder.WriteString(content.Text)
				}
			}
		}
		text = builder.String()
	}
	if strings.TrimSpace(text) == "" {
		if decoded.IncompleteDetails != nil {
			return "", decoded.Usage, fmt.Errorf("adaptation model response was incomplete: %s", decoded.IncompleteDetails.Reason)
		}
		return "", decoded.Usage, fmt.Errorf("adaptation model returned no output text (status %q)", decoded.Status)
	}
	return text, decoded.Usage, nil
}

func extractJSONObject(source string) (string, error) {
	trimmed := strings.TrimSpace(source)
	if strings.HasPrefix(trimmed, "```") {
		firstLine := strings.IndexByte(trimmed, '\n')
		lastFence := strings.LastIndex(trimmed, "```")
		if firstLine >= 0 && lastFence > firstLine {
			trimmed = strings.TrimSpace(trimmed[firstLine+1 : lastFence])
		}
	}
	start := strings.IndexByte(trimmed, '{')
	end := strings.LastIndexByte(trimmed, '}')
	if start < 0 || end <= start {
		return "", errors.New("model output did not contain a JSON object")
	}
	return trimmed[start : end+1], nil
}

func normalizeArtifact(artifact *Artifact) {
	if artifact.RequiredCapabilities == nil {
		artifact.RequiredCapabilities = []string{}
	}
	if artifact.State == nil {
		artifact.State = []StateDefinition{}
	}
	if artifact.MessageStateRules == nil {
		artifact.MessageStateRules = []MessageStateRule{}
	}
	for index := range artifact.MessageStateRules {
		if artifact.MessageStateRules[index].Mappings == nil {
			artifact.MessageStateRules[index].Mappings = []MessageStateMapping{}
		}
	}
	if artifact.Views == nil {
		artifact.Views = []View{}
	}
	if artifact.Report.RestoredBehaviors == nil {
		artifact.Report.RestoredBehaviors = []string{}
	}
	if artifact.Report.UnsupportedBehaviors == nil {
		artifact.Report.UnsupportedBehaviors = []string{}
	}
	if artifact.Report.Warnings == nil {
		artifact.Report.Warnings = []string{}
	}
	for viewIndex := range artifact.Views {
		view := &artifact.Views[viewIndex]
		if view.Nodes == nil {
			view.Nodes = []UINode{}
		}
		if view.SubmitActions == nil {
			view.SubmitActions = []Action{}
		}
		for nodeIndex := range view.Nodes {
			normalizeNode(&view.Nodes[nodeIndex])
		}
	}
}

func normalizeNode(node *UINode) {
	if node.Children == nil {
		node.Children = []UINode{}
	}
	if node.Fields == nil {
		node.Fields = []FormField{}
	}
	for childIndex := range node.Children {
		normalizeNode(&node.Children[childIndex])
	}
	for fieldIndex := range node.Fields {
		if node.Fields[fieldIndex].Options == nil {
			node.Fields[fieldIndex].Options = []FormOption{}
		}
	}
}

func repairPrompt(original, previous string, problem error) string {
	if len(previous) > MaxArtifactSize {
		previous = previous[:MaxArtifactSize]
	}
	return original + "\n\n你的上一次输出未通过确定性校验：" + problem.Error() +
		"\n请只修正 JSON 并重新输出。上一次输出如下：\n" + previous
}

func compilePrompt(programView string) string {
	return `将下面的 Program View 编译成 Tavern Player AdaptationArtifact v1。

只能恢复 Program View 中能观察到的交互意图；不得补写原卡叙事正文、世界书正文或任何被省略内容。不得输出 HTML、JavaScript、CSS、URL、data URI、file/content URI、网络动作或任意代码。无法表达的行为写入 report.unsupportedBehaviors，并将 status 设为 PARTIAL。

输出必须是且只能是一个 JSON 对象，完整使用这些字段：
{
  "schemaVersion": 1,
  "sourceSha256": "与 Program View 完全相同",
  "compiler": {"id":"tavern-shelf-ai","version":"2","model":null},
  "status": "FULL 或 PARTIAL",
  "requiredCapabilities": ["ui.native", "chat.setDraft", "state.write", "state.ingest" 中实际使用者],
  "state": [{"key":"lowercase-id","type":"STRING|NUMBER|BOOLEAN","initialValue":"与 type 匹配的 JSON primitive"}],
  "messageStateRules": [{"dialect":"UPDATE_VARIABLE_SET_V1","mappings":[{"sourcePath":"原始点分路径","target":"已声明 state key"}]}],
  "views": [{
    "id":"lowercase-id", "title":"", "placement":"MESSAGE_REPLACEMENT|MESSAGE_ATTACHMENT|CONVERSATION_HEADER",
    "trigger":{"type":"MESSAGE_EXACT|MESSAGE_CONTAINS|ALWAYS","value":"ALWAYS 时必须为空"},
    "nodes":[{"id":"lowercase-id","type":"SECTION|TEXT|STATUS|FORM","title":"","text":"","stateKey":null,"min":null,"max":null,"children":[],"fields":[]}],
    "submitLabel":null,
    "submitActions":[{"type":"CHAT_SET_DRAFT|STATE_SET|STATE_INCREMENT|STATE_TOGGLE","target":null,"value":null,"template":null}]
  }],
  "report":{"summary":"","restoredBehaviors":[],"unsupportedBehaviors":[],"warnings":[]}
}

FORM fields 使用 {"id":"lowercase-id","type":"TEXT|MULTILINE_TEXT|NUMBER|SINGLE_SELECT|MULTI_SELECT|TOGGLE","label":"","placeholder":"","required":false,"options":[{"value":"","label":""}],"initialValue":""}。
template 只允许 {{form.field-id}}、{{state.state-key}}、{{user}}、{{char}}。CHAT_SET_DRAFT 必须有 template 且没有 target/value。状态动作 target 必须引用已声明 state。所有 id 使用小写英数字和 ._- 分隔。

如果 Program View 包含 stateProtocolHints，请为 UI 实际读取的每个 values.path 声明 state，并生成 UPDATE_VARIABLE_SET_V1 messageStateRules 映射；初始值和类型沿用 hint，requiredCapabilities 声明 state.ingest。此方言只表示模型回复中 <UpdateVariable> 内的 _.set('点分路径', oldScalar, newScalar) 白名单更新，不代表执行 JavaScript。如果没有对应 hint，messageStateRules 必须为空。只声明实际使用的 capabilities，不得多声明。带 triggerPattern 的原始 markup 必须使用该 marker；triggerMatchMode 非空时必须原样作为 trigger.type，否则默认 MESSAGE_CONTAINS，不能使用 ALWAYS。原 markup 如果把数值状态展示为带 min/max 的进度条，使用 STATUS 节点表达，不要降级为普通 TEXT。hint 已提供的 initialValue 是可用的安全初始值，不得声称缺少初始化。

Program View：
` + programView
}

const compilerInstruction = "你是受限内容编译器。严格按照用户提供的 schema 将已脱敏交互程序映射为声明式原生 UI；只返回 JSON，不执行、转写或发明任何活跃代码。"
