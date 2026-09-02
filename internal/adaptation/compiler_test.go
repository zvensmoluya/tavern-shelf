package adaptation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompilerUsesResponsesAPIAndNormalizesProvenance(t *testing.T) {
	t.Parallel()
	sourceHash := strings.Repeat("a", 64)
	artifact := strings.Replace(validArtifactJSON, "test-model", "untrusted-model-name", 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected request: %s %s auth=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}
		var request responsesRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Model != "safe-model" || len(request.Input) != 2 || request.Store {
			t.Errorf("unexpected request body: %#v", request)
		}
		if request.Reasoning.Effort != "none" || request.Text.Format.Type != "json_object" {
			t.Errorf("JSON/no-reasoning constraints were not sent: %#v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"completed","output":[{"type":"reasoning","content":[]},{"type":"message","content":[{"type":"output_text","text":` + quoteJSON("```json\n"+artifact+"\n```") + `}]}],"usage":{"input_tokens":120,"output_tokens":80}}`))
	}))
	defer server.Close()
	compiler, err := NewCompiler(CompilerConfig{BaseURL: server.URL, APIKey: "test-key", Model: "safe-model", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	result, err := compiler.Compile(context.Background(), ProgramView{SchemaVersion: 1, SourceSHA256: sourceHash})
	if err != nil {
		t.Fatal(err)
	}
	if result.Attempts != 1 || result.InputTokens != 120 || result.OutputTokens != 80 {
		t.Fatalf("unexpected compile metrics: %#v", result)
	}
	if result.Artifact.Compiler.ID != CompilerID || result.Artifact.Compiler.Model == nil || *result.Artifact.Compiler.Model != "safe-model" {
		t.Fatalf("compiler provenance was not normalized: %#v", result.Artifact.Compiler)
	}
	decoded, _, err := DecodeArtifact(strings.NewReader(string(result.Raw)))
	if err != nil || len(ValidateArtifact(decoded, sourceHash)) != 0 {
		t.Fatalf("normalized result is not Player-compatible: %v", err)
	}
}

func TestCompilerRepairsInvalidFirstOutput(t *testing.T) {
	t.Parallel()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		output := `{"not":"an artifact"}`
		if calls == 2 {
			output = validArtifactJSON
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"completed","output_text":` + quoteJSON(output) + `}`))
	}))
	defer server.Close()
	compiler, err := NewCompiler(CompilerConfig{BaseURL: server.URL + "/", APIKey: "test-key", Model: "safe-model", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	result, err := compiler.Compile(context.Background(), ProgramView{SchemaVersion: 1, SourceSHA256: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || result.Attempts != 2 {
		t.Fatalf("repair attempts = %d/%d, want 2", calls, result.Attempts)
	}
}

func quoteJSON(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
