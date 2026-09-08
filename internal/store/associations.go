package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/zvensmoluya/tavern-shelf/internal/library"
)

func (s *Store) migrateAssociations(ctx context.Context) error {
	for _, column := range []string{
		"cover_hash TEXT NOT NULL DEFAULT ''", "content_hash TEXT NOT NULL DEFAULT ''", "identity_key TEXT NOT NULL DEFAULT ''",
		"group_id TEXT NOT NULL DEFAULT ''", "group_reason TEXT NOT NULL DEFAULT ''",
	} {
		if _, err := s.db.ExecContext(ctx, "ALTER TABLE characters ADD COLUMN "+column); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return fmt.Errorf("migrate revisions: %w", err)
		}
	}
	for _, column := range []string{"default_source", "shelf_cover"} {
		if _, err := s.db.ExecContext(ctx, "ALTER TABLE characters DROP COLUMN "+column); err != nil && !strings.Contains(strings.ToLower(err.Error()), "no such column") {
			return fmt.Errorf("remove superseded display choice: %w", err)
		}
	}
	_, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_character_content ON characters(content_hash);
 CREATE INDEX IF NOT EXISTS idx_character_identity ON characters(identity_key);
 CREATE INDEX IF NOT EXISTS idx_character_group ON characters(group_id);`)
	return err
}

func assignGroup(ctx context.Context, tx *sql.Tx, c *library.Character) error {
	c.GroupID, c.GroupReason = c.ID, "independent"
	// Multiple matches after a manual split are ambiguous. Do not silently undo
	// the user's decision or choose an arbitrary group.
	for _, match := range []struct{ column, value, reason string }{
		{"content_hash", c.ContentHash, "same-content"}, {"identity_key", c.IdentityKey, "name-creator"},
	} {
		if match.value == "" {
			continue
		}
		var count int
		var group sql.NullString
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(DISTINCT group_id), MIN(group_id) FROM characters WHERE "+match.column+" = ? AND group_id <> ''", match.value).Scan(&count, &group); err != nil {
			return fmt.Errorf("match character group: %w", err)
		}
		if count == 1 {
			c.GroupID, c.GroupReason = group.String, match.reason
			return nil
		}
		if count > 1 {
			c.GroupReason = "ambiguous"
			return nil
		}
	}
	return nil
}

// RebuildIdentity backfills derived fingerprints without changing explicit groups.
func (s *Store) RebuildIdentity(ctx context.Context, c library.Character) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if c.GroupID == "" {
		if err := assignGroup(ctx, tx, &c); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE characters SET content_hash=?, identity_key=?, group_id=?, group_reason=?, cover_hash=? WHERE id=?`, c.ContentHash, c.IdentityKey, c.GroupID, c.GroupReason, c.CoverHash, c.ID); err != nil {
		return fmt.Errorf("save character identity: %w", err)
	}
	return tx.Commit()
}

// ChangeAssociation only changes metadata; source files are never moved or rewritten.
// target is a source ID for a join, not an unvalidated group identifier.
func (s *Store) ChangeAssociation(ctx context.Context, id, action, target string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var group string
	if err := tx.QueryRowContext(ctx, `SELECT group_id FROM characters WHERE id=?`, id).Scan(&group); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	// An old row whose managed source could not be read may still lack a group.
	// It is an independent card, never part of a shared empty-string group.
	if group == "" {
		group = id
		if _, err := tx.ExecContext(ctx, `UPDATE characters SET group_id=?, group_reason='independent' WHERE id=?`, group, id); err != nil {
			return err
		}
	}
	switch action {
	case "split", "join":
		next := ""
		if action == "split" {
			var random [16]byte
			if _, err := rand.Read(random[:]); err != nil {
				return err
			}
			next = hex.EncodeToString(random[:])
		} else {
			if _, err := tx.ExecContext(ctx, `UPDATE characters SET group_id=id, group_reason='independent' WHERE id=? AND group_id=''`, target); err != nil {
				return err
			}
			if err := tx.QueryRowContext(ctx, `SELECT group_id FROM characters WHERE id=?`, target).Scan(&next); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return ErrNotFound
				}
				return err
			}
			if next == group {
				return tx.Commit()
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE characters SET group_id=?, group_reason='manual' WHERE id=?`, next, id); err != nil {
			return err
		}
	default:
		return errors.New("unknown revision action")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("save revision choice: %w", err)
	}
	return nil
}

// RestoreAssociation restores display associations without altering any original.
func (s *Store) RestoreAssociation(ctx context.Context, id, group, reason string) error {
	if group == "" {
		return nil
	}
	if len(group) > 128 || len(reason) > 64 {
		return errors.New("invalid restored group metadata")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE characters SET group_id=?, group_reason=? WHERE id=?`, group, reason, id)
	if err != nil {
		return fmt.Errorf("restore revision metadata: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrNotFound
	}
	return tx.Commit()
}
