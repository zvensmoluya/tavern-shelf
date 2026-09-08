package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/zvensmoluya/tavern-shelf/internal/library"
)

func TestUnrebuiltRowsDoNotShareEmptyGroup(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, id := range []string{"first", "second"} {
		if err := s.Create(ctx, library.Character{ID: id, SourceHash: id, Name: id, Tags: []string{}, SourceFormat: "json", ImportedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec(`UPDATE characters SET group_id=''`); err != nil {
		t.Fatal(err)
	}
	if err := s.ChangeAssociation(ctx, "first", "split", ""); err != nil {
		t.Fatal(err)
	}
	first, err := s.Get(ctx, "first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Get(ctx, "second")
	if err != nil {
		t.Fatal(err)
	}
	if first.GroupID == "" || second.GroupID != "" {
		t.Fatal("changing an unrebuilt row affected an unrelated card")
	}
	if err := s.ChangeAssociation(ctx, "first", "join", "second"); err != nil {
		t.Fatal(err)
	}
	first, _ = s.Get(ctx, "first")
	second, _ = s.Get(ctx, "second")
	if first.GroupID != "second" || second.GroupID != "second" {
		t.Fatal("join did not initialize the target group")
	}
}
