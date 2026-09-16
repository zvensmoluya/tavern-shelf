package importer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/zvensmoluya/tavern-shelf/internal/paths"
	"github.com/zvensmoluya/tavern-shelf/internal/store"
)

const validCard = `{"spec":"chara_card_v2","spec_version":"2.0","data":{"name":"Mara","creator":"Inkkeeper","tags":["fantasy"]}}`

func TestImportCopiesThenRemovesInboxSource(t *testing.T) {
	imp, p, s := testImporter(t)
	source := filepath.Join(p.Inbox, "mara.json")
	if err := os.WriteFile(source, []byte(validCard), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := imp.Import(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if result.Duplicate || result.Character.Name != "Mara" {
		t.Fatalf("unexpected import result: %#v", result)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("Inbox source still exists after committed import: %v", err)
	}
	managed := filepath.Join(p.Library, result.Character.SourceRelPath)
	if raw, err := os.ReadFile(managed); err != nil || string(raw) != validCard {
		t.Fatalf("managed source mismatch: %q, %v", raw, err)
	}
	characters, err := s.List(context.Background())
	if err != nil || len(characters) != 1 {
		t.Fatalf("library mismatch: %#v, %v", characters, err)
	}
}

func TestDuplicateIsArchivedWithoutCreatingAnotherCharacter(t *testing.T) {
	imp, p, s := testImporter(t)
	first := filepath.Join(p.Inbox, "first.json")
	if err := os.WriteFile(first, []byte(validCard), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := imp.Import(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	archivesBefore, _ := os.ReadDir(p.Duplicate)
	duplicate := filepath.Join(p.Inbox, "again.json")
	if err := os.WriteFile(duplicate, []byte(validCard), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := imp.Import(context.Background(), duplicate)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Duplicate {
		t.Fatal("expected duplicate result")
	}
	characters, _ := s.List(context.Background())
	if len(characters) != 1 {
		t.Fatalf("duplicate created another row: %d", len(characters))
	}
	duplicates, err := os.ReadDir(p.Duplicate)
	if err != nil || len(duplicates) != len(archivesBefore)+1 {
		t.Fatalf("duplicate was not archived: %v, %v", duplicates, err)
	}
}

func TestInvalidCardRemainsInInbox(t *testing.T) {
	imp, p, _ := testImporter(t)
	source := filepath.Join(p.Inbox, "broken.json")
	if err := os.WriteFile(source, []byte(`{"data":`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := imp.Import(context.Background(), source); err == nil {
		t.Fatal("expected invalid card to fail")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("invalid source was not preserved: %v", err)
	}
}

func TestImportFromConfiguredExternalInbox(t *testing.T) {
	imp, p, _ := testImporter(t)
	external := t.TempDir()
	source := filepath.Join(external, "external.json")
	if err := os.WriteFile(source, []byte(validCard), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := imp.ImportFrom(context.Background(), external, source)
	if err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(source); err != nil || string(raw) != validCard {
		t.Fatalf("external scan source was changed: %q, %v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(p.Library, result.Character.SourceRelPath)); err != nil {
		t.Fatalf("managed source missing: %v", err)
	}
}

func TestExternalDuplicateRemainsAtSource(t *testing.T) {
	imp, p, _ := testImporter(t)
	managedSource := filepath.Join(p.Inbox, "managed.json")
	if err := os.WriteFile(managedSource, []byte(validCard), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := imp.Import(context.Background(), managedSource); err != nil {
		t.Fatal(err)
	}
	archivesBefore, _ := os.ReadDir(p.Duplicate)
	external := t.TempDir()
	duplicate := filepath.Join(external, "duplicate.json")
	if err := os.WriteFile(duplicate, []byte(validCard), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := imp.ImportFrom(context.Background(), external, duplicate)
	if err != nil || !result.Duplicate {
		t.Fatalf("external duplicate result = %#v, %v", result, err)
	}
	if _, err := os.Stat(duplicate); err != nil {
		t.Fatalf("external duplicate was removed: %v", err)
	}
	duplicates, err := os.ReadDir(p.Duplicate)
	if err != nil || len(duplicates) != len(archivesBefore) {
		t.Fatalf("external duplicate was archived: %v, %v", duplicates, err)
	}
}

func TestImportFromRejectsFileOutsideConfiguredInbox(t *testing.T) {
	imp, _, _ := testImporter(t)
	external := t.TempDir()
	other := t.TempDir()
	source := filepath.Join(other, "outside.json")
	if err := os.WriteFile(source, []byte(validCard), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := imp.ImportFrom(context.Background(), external, source); err == nil {
		t.Fatal("expected source outside configured Inbox to be rejected")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("rejected source was changed: %v", err)
	}
}

func TestImportStandaloneWorldbook(t *testing.T) {
	imp, p, s := testImporter(t)
	source := filepath.Join(p.Inbox, "Eldoria.json")
	raw := `{"entries":{"0":{"uid":0,"key":["eldoria"],"comment":"Eldoria","content":"A magical forest.","disable":false}}}`
	if err := os.WriteFile(source, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := imp.Import(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "worldbook" || result.Resource.Name != "Eldoria" || result.Resource.Worldbook.EntryCount != 1 {
		t.Fatalf("unexpected worldbook import: %#v", result)
	}
	resources, err := s.ListResources(context.Background(), "worldbook")
	if err != nil || len(resources) != 1 {
		t.Fatalf("stored worldbooks mismatch: %#v, %v", resources, err)
	}
}

func TestImportPreset(t *testing.T) {
	imp, p, s := testImporter(t)
	source := filepath.Join(p.Inbox, "ChatML.json")
	raw := `{"name":"ChatML","input_sequence":"<|user|>","output_sequence":"<|assistant|>","stop_sequence":"</s>"}`
	if err := os.WriteFile(source, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := imp.Import(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "preset" || result.Resource.Subtype != "instruct" || result.Resource.Preset == nil {
		t.Fatalf("unexpected preset import: %#v", result)
	}
	resources, err := s.ListResources(context.Background(), "preset")
	if err != nil || len(resources) != 1 {
		t.Fatalf("stored presets mismatch: %#v, %v", resources, err)
	}
}

func TestImportBOMPresetPreservesSourceBytes(t *testing.T) {
	for _, external := range []bool{false, true} {
		name := "inbox"
		if external {
			name = "external"
		}
		t.Run(name, func(t *testing.T) {
			imp, p, s := testImporter(t)
			directory := p.Inbox
			if external {
				directory = t.TempDir()
			}
			importFile := func(source string) (Result, error) {
				if external {
					return imp.ImportFrom(context.Background(), directory, source)
				}
				return imp.Import(context.Background(), source)
			}
			raw := "\xef\xbb\xbf\r\n" + `{"prompts":[{"identifier":"main","content":"Write vividly."}],"prompt_order":[],"extensions":{"custom":true}}`
			for index, filename := range []string{"Community.json", "Duplicate.json"} {
				source := filepath.Join(directory, filename)
				if err := os.WriteFile(source, []byte(raw), 0o644); err != nil {
					t.Fatal(err)
				}
				result, err := importFile(source)
				if err != nil {
					t.Fatal(err)
				}
				if result.Kind != "preset" || result.Resource.Subtype != "openai" || result.Resource.Name != "Community" || result.Duplicate != (index > 0) {
					t.Fatalf("unexpected BOM preset: %#v", result)
				}
				managed := filepath.Join(p.Library, result.Resource.SourceRelPath)
				if saved, err := os.ReadFile(managed); err != nil || string(saved) != raw {
					t.Fatalf("managed source bytes changed: %q, %v", saved, err)
				}
				if external {
					if saved, err := os.ReadFile(source); err != nil || string(saved) != raw {
						t.Fatalf("external source bytes changed: %q, %v", saved, err)
					}
				} else if _, err := os.Stat(source); !os.IsNotExist(err) {
					t.Fatalf("committed source still in Inbox: %v", err)
				}
			}
			resources, err := s.ListResources(context.Background(), "preset")
			if err != nil || len(resources) != 1 {
				t.Fatalf("preset deduplication failed: %#v, %v", resources, err)
			}
		})
	}
}

func TestInvalidBOMResourceRemainsInInbox(t *testing.T) {
	for _, raw := range []string{
		"\xef\xbb\xbf" + `{"prompts":[],"prompt_order":`,
		"\xef\xbb\xbf" + `{"theme":"dark"}`,
	} {
		imp, p, s := testImporter(t)
		source := filepath.Join(p.Inbox, "unsupported.json")
		if err := os.WriteFile(source, []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := imp.Import(context.Background(), source); err == nil {
			t.Fatal("expected invalid resource to fail")
		}
		if saved, err := os.ReadFile(source); err != nil || string(saved) != raw {
			t.Fatalf("rejected source bytes changed: %q, %v", saved, err)
		}
		resources, err := s.ListResources(context.Background(), "")
		if err != nil || len(resources) != 0 {
			t.Fatalf("invalid resource entered Library: %#v, %v", resources, err)
		}
	}
}

func TestEmbeddedWorldbookRemainsPartOfCharacter(t *testing.T) {
	imp, p, s := testImporter(t)
	source := filepath.Join(p.Inbox, "mara-with-lore.json")
	raw := `{"spec":"chara_card_v2","data":{"name":"Mara","character_book":{"name":"Mara lore","entries":[{"keys":["mara"],"content":"Lore","enabled":true}]}}}`
	if err := os.WriteFile(source, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := imp.Import(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "character" || !result.Character.HasWorldbook {
		t.Fatalf("embedded lore was not kept on the character: %#v", result)
	}
	resources, err := s.ListResources(context.Background(), "worldbook")
	if err != nil || len(resources) != 0 {
		t.Fatalf("embedded lore created a standalone worldbook: %#v, %v", resources, err)
	}
}

func testImporter(t *testing.T) (*Importer, paths.Paths, *store.Store) {
	t.Helper()
	p, err := paths.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(p.Database)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return New(p, s), p, s
}
