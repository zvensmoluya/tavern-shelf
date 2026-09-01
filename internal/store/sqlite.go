package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/openai/tavern-shelf/internal/library"
	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("character not found")

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	dsn := "file:" + filepath.ToSlash(abs) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open library database: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS characters (
    id TEXT PRIMARY KEY,
    source_hash TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    creator TEXT NOT NULL DEFAULT '',
    spec TEXT NOT NULL DEFAULT '',
    spec_version TEXT NOT NULL DEFAULT '',
    tags_json TEXT NOT NULL DEFAULT '[]',
    has_worldbook INTEGER NOT NULL DEFAULT 0,
    has_regex INTEGER NOT NULL DEFAULT 0,
    has_extensions INTEGER NOT NULL DEFAULT 0,
    has_interactive INTEGER NOT NULL DEFAULT 0,
    source_format TEXT NOT NULL,
    source_is_image INTEGER NOT NULL DEFAULT 0,
    source_filename TEXT NOT NULL,
    source_rel_path TEXT NOT NULL,
    source_size INTEGER NOT NULL,
    imported_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_characters_imported_at ON characters(imported_at DESC);
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate library database: %w", err)
	}
	return nil
}

func (s *Store) Create(ctx context.Context, character library.Character) error {
	tags, err := json.Marshal(character.Tags)
	if err != nil {
		return fmt.Errorf("encode tags: %w", err)
	}
	const query = `INSERT INTO characters (
id, source_hash, name, creator, spec, spec_version, tags_json,
has_worldbook, has_regex, has_extensions, has_interactive,
source_format, source_is_image, source_filename, source_rel_path,
source_size, imported_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = s.db.ExecContext(ctx, query,
		character.ID, character.SourceHash, character.Name, character.Creator,
		character.Spec, character.SpecVersion, string(tags), character.HasWorldbook,
		character.HasRegex, character.HasExtensions, character.HasInteractive,
		character.SourceFormat, character.SourceIsImage, character.SourceFilename,
		character.SourceRelPath, character.SourceSize, character.ImportedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("insert character: %w", err)
	}
	return nil
}

func (s *Store) List(ctx context.Context) ([]library.Character, error) {
	const query = `SELECT id, source_hash, name, creator, spec, spec_version, tags_json,
has_worldbook, has_regex, has_extensions, has_interactive, source_format,
source_is_image, source_filename, source_rel_path, source_size, imported_at
FROM characters ORDER BY imported_at DESC, name COLLATE NOCASE`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list characters: %w", err)
	}
	defer rows.Close()
	characters := make([]library.Character, 0)
	for rows.Next() {
		character, err := scanCharacter(rows)
		if err != nil {
			return nil, err
		}
		characters = append(characters, character)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate characters: %w", err)
	}
	return characters, nil
}

func (s *Store) Get(ctx context.Context, id string) (library.Character, error) {
	const query = `SELECT id, source_hash, name, creator, spec, spec_version, tags_json,
has_worldbook, has_regex, has_extensions, has_interactive, source_format,
source_is_image, source_filename, source_rel_path, source_size, imported_at
FROM characters WHERE id = ?`
	character, err := scanCharacter(s.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return library.Character{}, ErrNotFound
	}
	return character, err
}

func (s *Store) GetByHash(ctx context.Context, hash string) (library.Character, error) {
	const query = `SELECT id, source_hash, name, creator, spec, spec_version, tags_json,
has_worldbook, has_regex, has_extensions, has_interactive, source_format,
source_is_image, source_filename, source_rel_path, source_size, imported_at
FROM characters WHERE source_hash = ?`
	character, err := scanCharacter(s.db.QueryRowContext(ctx, query, hash))
	if errors.Is(err, sql.ErrNoRows) {
		return library.Character{}, ErrNotFound
	}
	return character, err
}

func (s *Store) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM characters WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete character: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted characters: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCharacter(row scanner) (library.Character, error) {
	var character library.Character
	var tags, imported string
	var worldbook, regex, extensions, interactive, image bool
	err := row.Scan(
		&character.ID, &character.SourceHash, &character.Name, &character.Creator,
		&character.Spec, &character.SpecVersion, &tags, &worldbook, &regex,
		&extensions, &interactive, &character.SourceFormat, &image,
		&character.SourceFilename, &character.SourceRelPath, &character.SourceSize, &imported,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Character{}, err
		}
		return library.Character{}, fmt.Errorf("scan character: %w", err)
	}
	if err := json.Unmarshal([]byte(tags), &character.Tags); err != nil {
		return library.Character{}, fmt.Errorf("decode stored tags: %w", err)
	}
	character.ImportedAt, err = time.Parse(time.RFC3339Nano, imported)
	if err != nil {
		return library.Character{}, fmt.Errorf("decode imported time: %w", err)
	}
	character.HasWorldbook = worldbook
	character.HasRegex = regex
	character.HasExtensions = extensions
	character.HasInteractive = interactive
	character.SourceIsImage = image
	return character, nil
}

func IsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}
