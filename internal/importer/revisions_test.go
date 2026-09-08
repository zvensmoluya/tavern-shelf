package importer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zvensmoluya/tavern-shelf/internal/library"
)

func TestRevisionsKeepEveryOriginalAndRespectSplits(t *testing.T) {
	imp, p, s := testImporter(t)
	ctx := context.Background()
	importRaw := func(name string, raw []byte) library.Character {
		t.Helper()
		path := filepath.Join(p.Inbox, name)
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		result, err := imp.Import(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		stored, err := os.ReadFile(filepath.Join(p.Library, result.Character.SourceRelPath))
		if err != nil || !bytes.Equal(stored, raw) {
			t.Fatal("original changed")
		}
		return result.Character
	}
	first := importRaw("first.json", []byte(validCard))
	png := importRaw("cover.png", importPNG(t, validCard))
	if png.GroupID != first.GroupID || png.ContentHash != first.ContentHash || png.GroupReason != "same-content" {
		t.Fatal("same data in PNG was not grouped")
	}
	revised := importRaw("revised.json", []byte(strings.Replace(validCard, "fantasy", "fantasy revised", 1)))
	if revised.GroupID != first.GroupID || revised.ContentHash == first.ContentHash || revised.GroupReason != "name-creator" {
		t.Fatal("content revision did not join card")
	}
	if err := s.ChangeAssociation(ctx, revised.ID, "split", ""); err != nil {
		t.Fatal(err)
	}
	split, _ := s.Get(ctx, revised.ID)
	if split.GroupID == first.GroupID {
		t.Fatal("split did not reset membership and preferences")
	}
	next := importRaw("next.json", []byte(strings.Replace(validCard, "fantasy", "fantasy next", 1)))
	if next.GroupID == first.GroupID || next.GroupID == split.GroupID || next.GroupReason != "ambiguous" {
		t.Fatal("ambiguous import undid manual split")
	}
	if err := s.ChangeAssociation(ctx, revised.ID, "join", first.ID); err != nil {
		t.Fatal(err)
	}
	joined, _ := s.Get(ctx, revised.ID)
	if joined.GroupID != first.GroupID {
		t.Fatal("manual join failed")
	}
	before := joined
	if err := s.ChangeAssociation(ctx, revised.ID, "join", "missing"); err == nil {
		t.Fatal("invalid target accepted")
	}
	after, _ := s.Get(ctx, revised.ID)
	if after.GroupID != before.GroupID {
		t.Fatal("failed join changed metadata")
	}
	foreign := importRaw("foreign.json", []byte(strings.Replace(validCard, "Inkkeeper", "Other author", 1)))
	if foreign.GroupID == first.GroupID {
		t.Fatal("same name with different creator merged")
	}
}
