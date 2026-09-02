package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zvensmoluya/tavern-shelf/internal/library"
)

func (s *Store) CharacterOrganizations(ctx context.Context) (map[string]library.CharacterOrganization, error) {
	result := make(map[string]library.CharacterOrganization)
	rows, err := s.db.QueryContext(ctx, `SELECT character_id, favorite, note FROM character_organization`)
	if err != nil {
		return nil, fmt.Errorf("list character organization: %w", err)
	}
	for rows.Next() {
		var id string
		var organization library.CharacterOrganization
		if err := rows.Scan(&id, &organization.Favorite, &organization.Note); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan character organization: %w", err)
		}
		organization.CollectionIDs = []string{}
		result[id] = organization
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close character organization rows: %w", err)
	}
	rows, err = s.db.QueryContext(ctx, `SELECT character_id, collection_id FROM collection_characters ORDER BY added_at, collection_id`)
	if err != nil {
		return nil, fmt.Errorf("list character collection assignments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var characterID, collectionID string
		if err := rows.Scan(&characterID, &collectionID); err != nil {
			return nil, fmt.Errorf("scan character collection assignment: %w", err)
		}
		organization := result[characterID]
		organization.CollectionIDs = append(organization.CollectionIDs, collectionID)
		result[characterID] = organization
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate character collection assignments: %w", err)
	}
	return result, nil
}

func (s *Store) CharacterOrganization(ctx context.Context, characterID string) (library.CharacterOrganization, error) {
	if _, err := s.Get(ctx, characterID); err != nil {
		return library.CharacterOrganization{}, err
	}
	organizations, err := s.CharacterOrganizations(ctx)
	if err != nil {
		return library.CharacterOrganization{}, err
	}
	organization := organizations[characterID]
	if organization.CollectionIDs == nil {
		organization.CollectionIDs = []string{}
	}
	return organization, nil
}

func (s *Store) SaveCharacterOrganization(ctx context.Context, characterID string, organization library.CharacterOrganization) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin character organization update: %w", err)
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM characters WHERE id = ?`, characterID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("find character for organization update: %w", err)
	}
	unique := make([]string, 0, len(organization.CollectionIDs))
	seen := make(map[string]struct{}, len(organization.CollectionIDs))
	for _, id := range organization.CollectionIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM collections WHERE id = ?`, id).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("collection %q: %w", id, ErrNotFound)
		} else if err != nil {
			return fmt.Errorf("validate collection assignment: %w", err)
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO character_organization (character_id, favorite, note, updated_at)
VALUES (?, ?, ?, ?) ON CONFLICT(character_id) DO UPDATE SET favorite=excluded.favorite, note=excluded.note, updated_at=excluded.updated_at`,
		characterID, organization.Favorite, organization.Note, now); err != nil {
		return fmt.Errorf("save character organization: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM collection_characters WHERE character_id = ?`, characterID); err != nil {
		return fmt.Errorf("clear character collection assignments: %w", err)
	}
	for _, collectionID := range unique {
		if _, err := tx.ExecContext(ctx, `INSERT INTO collection_characters (collection_id, character_id, added_at) VALUES (?, ?, ?)`, collectionID, characterID, now); err != nil {
			return fmt.Errorf("save character collection assignment: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit character organization update: %w", err)
	}
	return nil
}

func (s *Store) ListCollections(ctx context.Context) ([]library.Collection, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.id, c.name, c.created_at, COUNT(ch.id)
FROM collections c
LEFT JOIN collection_characters cc ON cc.collection_id = c.id
LEFT JOIN characters ch ON ch.id = cc.character_id
GROUP BY c.id, c.name, c.created_at
ORDER BY c.name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	defer rows.Close()
	collections := make([]library.Collection, 0)
	for rows.Next() {
		var collection library.Collection
		var createdAt string
		if err := rows.Scan(&collection.ID, &collection.Name, &createdAt, &collection.CharacterCount); err != nil {
			return nil, fmt.Errorf("scan collection: %w", err)
		}
		collection.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("decode collection creation time: %w", err)
		}
		collections = append(collections, collection)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate collections: %w", err)
	}
	return collections, nil
}

func (s *Store) CreateCollection(ctx context.Context, collection library.Collection) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO collections (id, name, created_at) VALUES (?, ?, ?)`, collection.ID, collection.Name, collection.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create collection: %w", err)
	}
	return nil
}

func (s *Store) RenameCollection(ctx context.Context, id, name string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE collections SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return fmt.Errorf("rename collection: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect collection rename: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteCollection(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM collections WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete collection: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect collection delete: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
