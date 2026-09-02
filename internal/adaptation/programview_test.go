package adaptation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExtractProgramViewKeepsOnlySanitizedProgramSurface(t *testing.T) {
	t.Parallel()
	raw := `{
  "spec":"chara_card_v3",
  "spec_version":"3.0",
  "data":{
    "name":"Safe Test",
    "description":"PRIVATE NARRATIVE",
    "first_mes":"PRIVATE GREETING\n<FORM_PLACEHOLDER/>",
    "character_book":{"entries":[
      {"id":7,"comment":"Private place","content":"PRIVATE WORLD BOOK","enabled":true}
    ]},
    "extensions":{
      "regex_scripts":[{
        "scriptName":"Opening form",
        "findRegex":"<FORM_PLACEHOLDER/>",
        "replaceString":"<form><img src=\"data:image/png;base64,AAAA\"><script>const api_key='super-secret'; fetch('https://user:pass@example.test/ui.js?token=private'); getvar('affection'); document.querySelector('#send_textarea'); const p='C:\\\\Users\\\\alice\\\\secret.txt';</script></form>",
        "placement":[1],
        "disabled":false
      }],
      "TavernHelper_scripts":[{
        "type":"script",
        "name":"State bridge",
        "content":"set_message_variable('mood', 'calm'); eventOn('MESSAGE_RECEIVED', () => {});",
        "enabled":true
      }]
    }
  }
}`
	path := filepath.Join(t.TempDir(), "card.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(raw))
	sourceHash := hex.EncodeToString(sum[:])

	view, err := ExtractProgramView(path, sourceHash)
	if err != nil {
		t.Fatal(err)
	}
	if view.SchemaVersion != 1 || view.SourceSHA256 != sourceHash {
		t.Fatalf("unexpected identity: %#v", view)
	}
	if len(view.ProgramBlocks) != 2 {
		t.Fatalf("program block count = %d, want 2: %#v", len(view.ProgramBlocks), view.ProgramBlocks)
	}
	if view.ProgramBlocks[0].TriggerMatchMode != "MESSAGE_CONTAINS" {
		t.Fatalf("trigger match mode = %q, want MESSAGE_CONTAINS", view.ProgramBlocks[0].TriggerMatchMode)
	}
	if len(view.WorldBookHandles) != 1 || view.WorldBookHandles[0].Handle != "worldbook:0:7" {
		t.Fatalf("world-book handles were not preserved: %#v", view.WorldBookHandles)
	}
	if view.WorldBookHandles[0].ContentChars == 0 || view.WorldBookHandles[0].ContentSHA256 == "" {
		t.Fatalf("world-book opaque metadata is incomplete: %#v", view.WorldBookHandles[0])
	}

	encoded := mustJSON(t, view)
	for _, forbidden := range []string{
		"PRIVATE NARRATIVE", "PRIVATE GREETING", "PRIVATE WORLD BOOK", "super-secret",
		"user:pass", "token=private", "AAAA", `C:\\Users\\alice`,
	} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("program view leaked %q: %s", forbidden, encoded)
		}
	}
	if !strings.Contains(encoded, "dependency://dependency-1") {
		t.Fatalf("remote reference was not replaced with an opaque dependency: %s", encoded)
	}
	if len(view.Dependencies) != 1 || view.Dependencies[0].Locator != "https://example.test/ui.js" {
		t.Fatalf("dependency was not sanitized: %#v", view.Dependencies)
	}
	if !reflect.DeepEqual(view.ObservedCapabilities, []string{"chat.write", "event.subscribe", "state.read", "state.write"}) {
		t.Fatalf("unexpected capabilities: %#v", view.ObservedCapabilities)
	}
	if !reflect.DeepEqual(view.ReferencedVariables, []string{"affection", "mood"}) {
		t.Fatalf("unexpected variable references: %#v", view.ReferencedVariables)
	}
	if len(view.OmittedContent) != 2 {
		t.Fatalf("omitted content metadata = %#v, want description and first message", view.OmittedContent)
	}
	wantRedactions := map[string]bool{"inline-data": true, "credential": true, "local-path": true}
	for _, redaction := range view.Redactions {
		delete(wantRedactions, redaction.Kind)
	}
	if len(wantRedactions) != 0 {
		t.Fatalf("missing redaction kinds: %#v (got %#v)", wantRedactions, view.Redactions)
	}
}

func TestExtractProgramViewRejectsInvalidSourceHash(t *testing.T) {
	t.Parallel()
	if _, err := ExtractProgramView("unused.json", "not-a-hash"); err == nil {
		t.Fatal("expected invalid source hash to fail before reading the card")
	}
}

func TestExtractProgramViewReducesUpdateVariableWorldBookToPrimitiveStateHints(t *testing.T) {
	t.Parallel()
	raw := `{
  "spec":"chara_card_v3",
  "data":{
    "name":"State Test",
    "character_book":{"entries":[
      {"comment":"旧变量更新规则","content":"<UpdateVariable>\n_.set('停用.路径', 0, 1);\n</UpdateVariable>","enabled":false},
      {"comment":"变量更新规则","content":"<status_current_variables>{{get_message_variable::stat_data}}</status_current_variables>\n<UpdateVariable>\n_.set('${path_of_changed_variable}', 0, 1);\n_.set('世界.日期', 1, 2);\n</UpdateVariable>","enabled":true},
      {"comment":"[InitVar]损坏副本","content":"{\"泄漏\":[\"TRAILING PRIVATE\",\"description\"]} trailing","enabled":false},
      {"comment":"[InitVar]初始化","content":"{\"世界\":{\"日期\":[1,\"PRIVATE DESCRIPTION\"],\"地点\":[\"https://user:pass@example.test/private?token=secret\",\"PRIVATE LOCATION DESCRIPTION\"]},\"角色\":{\"好感度\":[50,\"PRIVATE RELATIONSHIP DESCRIPTION\"]}}","enabled":false}
    ]},
    "extensions":{}
  }
}`
	path := filepath.Join(t.TempDir(), "card.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(raw))
	view, err := ExtractProgramView(path, hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatal(err)
	}
	if len(view.StateProtocolHints) != 1 {
		t.Fatalf("state protocol hints = %#v", view.StateProtocolHints)
	}
	hint := view.StateProtocolHints[0]
	if hint.Dialect != "UPDATE_VARIABLE_SET_V1" || hint.VariableName != "stat_data" || len(hint.Values) != 3 {
		t.Fatalf("unexpected protocol hint: %#v", hint)
	}
	values := map[string]StateValueHint{}
	for _, value := range hint.Values {
		values[value.Path] = value
	}
	if date := values["世界.日期"]; string(date.InitialValue) != `1` || date.Type != "NUMBER" {
		t.Fatalf("unexpected date state hint: %#v", date)
	}
	encoded := mustJSON(t, view)
	for _, forbidden := range []string{"PRIVATE DESCRIPTION", "PRIVATE LOCATION DESCRIPTION", "PRIVATE RELATIONSHIP DESCRIPTION", "TRAILING PRIVATE", "user:pass", "token=secret"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("state hint leaked %q: %s", forbidden, encoded)
		}
	}
	if !strings.Contains(encoded, `"initialValue":"dependency://dependency-1"`) {
		t.Fatalf("state string was not sanitized: %s", encoded)
	}
	if !reflect.DeepEqual(view.ObservedCapabilities, []string{"state.read", "state.write"}) {
		t.Fatalf("unexpected state capabilities: %#v", view.ObservedCapabilities)
	}
	if !reflect.DeepEqual(view.ReferencedVariables, []string{"stat_data"}) {
		t.Fatalf("unexpected variables: %#v", view.ReferencedVariables)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
