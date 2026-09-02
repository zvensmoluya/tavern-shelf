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
    "first_mes":"PRIVATE GREETING",
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

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
