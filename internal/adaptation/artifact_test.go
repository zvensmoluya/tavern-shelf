package adaptation

import (
	"strings"
	"testing"
)

const validArtifactJSON = `{
  "schemaVersion":1,
  "sourceSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "compiler":{"id":"tavern-shelf-ai","version":"1","model":"test-model"},
  "status":"FULL",
  "requiredCapabilities":["ui.native","chat.setDraft","state.write"],
  "state":[{"key":"visits","type":"NUMBER","initialValue":0}],
  "views":[{
    "id":"opening-form",
    "title":"预约",
    "placement":"MESSAGE_REPLACEMENT",
    "trigger":{"type":"MESSAGE_EXACT","value":"<FORM_PLACEHOLDER/>"},
    "nodes":[{"id":"form","type":"FORM","title":"预约信息","text":"","children":[],"fields":[
      {"id":"name","type":"TEXT","label":"姓名","placeholder":"","required":true,"options":[],"initialValue":""}
    ]}],
    "submitLabel":"写入消息",
    "submitActions":[
      {"type":"STATE_INCREMENT","target":"visits","value":"1"},
      {"type":"CHAT_SET_DRAFT","template":"姓名：{{form.name}}；预约次数：{{state.visits}}"}
    ]
  }],
  "report":{"summary":"原生预约表单","restoredBehaviors":["填写预约"],"unsupportedBehaviors":[],"warnings":[]}
}`

func TestDecodeAndValidateArtifact(t *testing.T) {
	t.Parallel()
	artifact, raw, err := DecodeArtifact(strings.NewReader(validArtifactJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("artifact bytes were not returned")
	}
	if issues := ValidateArtifact(artifact, artifact.SourceSHA256); len(issues) != 0 {
		t.Fatalf("valid artifact issues: %#v", issues)
	}
}

func TestDecodeArtifactRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	invalid := strings.Replace(validArtifactJSON, `"status":"FULL"`, `"status":"FULL","script":"alert(1)"`, 1)
	if _, _, err := DecodeArtifact(strings.NewReader(invalid)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %v, want unknown field rejection", err)
	}
}

func TestValidateArtifactRejectsUnsafeAndUnboundBehavior(t *testing.T) {
	t.Parallel()
	artifact, _, err := DecodeArtifact(strings.NewReader(validArtifactJSON))
	if err != nil {
		t.Fatal(err)
	}
	artifact.SourceSHA256 = strings.Repeat("b", 64)
	artifact.RequiredCapabilities = []string{"ui.native", "network.fetch"}
	unsafe := "https://example.test/{{form.missing}}"
	artifact.Views[0].SubmitActions[1].Template = &unsafe
	issues := ValidateArtifact(artifact, strings.Repeat("a", 64))
	want := map[string]bool{
		"SOURCE_HASH_MISMATCH":       true,
		"UNSUPPORTED_CAPABILITY":     true,
		"MISSING_CAPABILITY":         true,
		"EXTERNAL_IO_FORBIDDEN":      true,
		"UNKNOWN_TEMPLATE_REFERENCE": true,
	}
	for _, issue := range issues {
		delete(want, issue.Code)
	}
	if len(want) != 0 {
		t.Fatalf("missing issue codes %#v; got %#v", want, issues)
	}
}
