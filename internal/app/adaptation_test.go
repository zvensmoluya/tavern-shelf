package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const adaptationTestCard = `{
  "spec":"chara_card_v3",
  "spec_version":"3.0",
  "data":{
    "name":"Native Candidate",
    "description":"Original narrative stays here.",
    "first_mes":"<OPENING_FORM/>",
    "extensions":{"regex_scripts":[{
      "scriptName":"Opening form",
      "findRegex":"<OPENING_FORM/>",
      "replaceString":"<form><input id='name'><button onclick=\"document.querySelector('#send_textarea').value='Name'\">Send</button></form>",
      "placement":[1]
    }]}
  }
}`

func TestAdaptationPipelinePreservesOriginalAndBindsDerivedFiles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	shelf, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	imported, err := shelf.ImportUpload(ctx, "candidate.json", bytes.NewBufferString(adaptationTestCard))
	if err != nil {
		t.Fatal(err)
	}
	character := imported.Character
	sourcePath := filepath.Join(shelf.Paths.Library, character.SourceRelPath)
	original, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}

	view, record, err := shelf.BuildProgramView(ctx, character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.SourceSHA256 != character.SourceHash || len(view.ProgramBlocks) != 1 {
		t.Fatalf("unexpected Program View: %#v", view)
	}
	if record.Status != "PROGRAM_VIEW_READY" || record.ProgramViewHash == "" {
		t.Fatalf("unexpected adaptation record: %#v", record)
	}
	if _, err := os.Stat(filepath.Join(shelf.Paths.Library, record.ProgramViewPath)); err != nil {
		t.Fatalf("Program View was not stored: %v", err)
	}

	artifactJSON := fmt.Sprintf(`{
  "schemaVersion":1,
  "sourceSha256":%q,
  "compiler":{"id":"test-compiler","version":"1","model":"fixture"},
  "status":"FULL",
  "requiredCapabilities":["ui.native","chat.setDraft"],
  "state":[],
  "views":[{
    "id":"opening-form","title":"开始","placement":"MESSAGE_REPLACEMENT",
    "trigger":{"type":"MESSAGE_EXACT","value":"<OPENING_FORM/>"},
    "nodes":[{"id":"form","type":"FORM","title":"资料","text":"","children":[],"fields":[
      {"id":"name","type":"TEXT","label":"姓名","placeholder":"","required":true,"options":[],"initialValue":""}
    ]}],
    "submitLabel":"写入消息",
    "submitActions":[{"type":"CHAT_SET_DRAFT","template":"姓名：{{form.name}}"}]
  }],
  "report":{"summary":"原生表单","restoredBehaviors":["填写姓名"],"unsupportedBehaviors":[],"warnings":[]}
}`, character.SourceHash)
	record, err = shelf.InstallAdaptation(ctx, character.ID, strings.NewReader(artifactJSON))
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != "FULL" || record.CompilerModel != "fixture" || record.ArtifactSize != int64(len(artifactJSON)) {
		t.Fatalf("unexpected installed adaptation: %#v", record)
	}
	stored, path, err := shelf.AdaptationPath(ctx, character.ID)
	if err != nil || stored.ArtifactHash != record.ArtifactHash {
		t.Fatalf("could not reload adaptation: %#v, %v", stored, err)
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != artifactJSON {
		t.Fatalf("stored adaptation mismatch: %q, %v", raw, err)
	}
	if after, err := os.ReadFile(sourcePath); err != nil || !bytes.Equal(after, original) {
		t.Fatalf("adaptation changed original source: %q, %v", after, err)
	}
	transferSource, err := shelf.resolveTransferSource(ctx, "character", character.ID)
	if err != nil || transferSource.Adaptation == nil || transferSource.Adaptation.SHA256 != record.ArtifactHash {
		t.Fatalf("transfer did not attach the validated adaptation: %#v, %v", transferSource.Adaptation, err)
	}
	if _, err := shelf.InstallAdaptation(ctx, character.ID, strings.NewReader(strings.Replace(artifactJSON, character.SourceHash, strings.Repeat("b", 64), 1))); err == nil {
		t.Fatal("expected an artifact for another source to be rejected")
	}
}

func TestBuildProgramViewRejectsChangedManagedSource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	shelf, err := Open(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shelf.Close() })
	imported, err := shelf.ImportUpload(ctx, "candidate.json", bytes.NewBufferString(adaptationTestCard))
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(shelf.Paths.Library, imported.Character.SourceRelPath)
	if err := os.WriteFile(sourcePath, []byte(`{"changed":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := shelf.BuildProgramView(ctx, imported.Character.ID); err == nil || !strings.Contains(err.Error(), "immutable source hash") {
		t.Fatalf("error = %v, want immutable source mismatch", err)
	}
}
