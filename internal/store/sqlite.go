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

	"github.com/zvensmoluya/tavern-shelf/internal/library"
	"github.com/zvensmoluya/tavern-shelf/internal/manifest"
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
    imported_at TEXT NOT NULL,
    manifest_json TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_characters_imported_at ON characters(imported_at DESC);
CREATE TABLE IF NOT EXISTS inbox_directories (
    path TEXT PRIMARY KEY,
    position INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS resources (
    id TEXT PRIMARY KEY,
    source_hash TEXT NOT NULL UNIQUE,
    kind TEXT NOT NULL,
    subtype TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    source_filename TEXT NOT NULL,
    source_rel_path TEXT NOT NULL,
    source_size INTEGER NOT NULL,
    imported_at TEXT NOT NULL,
    details_json TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_resources_kind_imported_at ON resources(kind, imported_at DESC);
CREATE TABLE IF NOT EXISTS character_organization (
    character_id TEXT PRIMARY KEY,
    favorite INTEGER NOT NULL DEFAULT 0,
    note TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS collections (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE,
    created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS collection_characters (
    collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    character_id TEXT NOT NULL,
    added_at TEXT NOT NULL,
    PRIMARY KEY (collection_id, character_id)
);
CREATE INDEX IF NOT EXISTS idx_collection_characters_character ON collection_characters(character_id);
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate library database: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "ALTER TABLE characters ADD COLUMN manifest_json TEXT NOT NULL DEFAULT '{}'"); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return fmt.Errorf("add content manifest column: %w", err)
	}
	return nil
}

func (s *Store) InboxDirectories(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT path FROM inbox_directories ORDER BY position, path")
	if err != nil {
		return nil, fmt.Errorf("list Inbox directories: %w", err)
	}
	defer rows.Close()
	directories := make([]string, 0)
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("scan Inbox directory: %w", err)
		}
		directories = append(directories, path)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Inbox directories: %w", err)
	}
	return directories, nil
}

func (s *Store) ReplaceInboxDirectories(ctx context.Context, directories []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Inbox settings update: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM inbox_directories"); err != nil {
		return fmt.Errorf("clear Inbox settings: %w", err)
	}
	for position, directory := range directories {
		if _, err := tx.ExecContext(ctx, "INSERT INTO inbox_directories (path, position) VALUES (?, ?)", directory, position); err != nil {
			return fmt.Errorf("save Inbox directory %q: %w", directory, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Inbox settings: %w", err)
	}
	return nil
}

func (s *Store) Create(ctx context.Context, character library.Character) error {
	tags, err := json.Marshal(character.Tags)
	if err != nil {
		return fmt.Errorf("encode tags: %w", err)
	}
	contentManifest, err := json.Marshal(character.Manifest)
	if err != nil {
		return fmt.Errorf("encode content manifest: %w", err)
	}
	const query = `INSERT INTO characters (
id, source_hash, name, creator, spec, spec_version, tags_json,
has_worldbook, has_regex, has_extensions, has_interactive,
source_format, source_is_image, source_filename, source_rel_path,
source_size, imported_at, manifest_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = s.db.ExecContext(ctx, query,
		character.ID, character.SourceHash, character.Name, character.Creator,
		character.Spec, character.SpecVersion, string(tags), character.HasWorldbook,
		character.HasRegex, character.HasExtensions, character.HasInteractive,
		character.SourceFormat, character.SourceIsImage, character.SourceFilename,
		character.SourceRelPath, character.SourceSize, character.ImportedAt.UTC().Format(time.RFC3339Nano),
		string(contentManifest),
	)
	if err != nil {
		return fmt.Errorf("insert character: %w", err)
	}
	return nil
}

func (s *Store) CreateResource(ctx context.Context, resource library.Resource) error {
	details, err := json.Marshal(struct {
		Worldbook *manifest.CharacterBook `json:"worldbook,omitempty"`
		Preset    *manifest.Preset        `json:"preset,omitempty"`
	}{Worldbook: resource.Worldbook, Preset: resource.Preset})
	if err != nil {
		return fmt.Errorf("encode resource details: %w", err)
	}
	const query = `INSERT INTO resources (
id, source_hash, kind, subtype, name, description, source_filename,
source_rel_path, source_size, imported_at, details_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = s.db.ExecContext(ctx, query,
		resource.ID, resource.SourceHash, resource.Kind, resource.Subtype, resource.Name,
		resource.Description, resource.SourceFilename, resource.SourceRelPath,
		resource.SourceSize, resource.ImportedAt.UTC().Format(time.RFC3339Nano), string(details),
	)
	if err != nil {
		return fmt.Errorf("insert resource: %w", err)
	}
	return nil
}

func (s *Store) SetImportedAt(ctx context.Context, kind, id string, importedAt time.Time) error {
	table := "resources"
	if kind == "character" {
		table = "characters"
	}
	result, err := s.db.ExecContext(ctx, "UPDATE "+table+" SET imported_at = ? WHERE id = ?", importedAt.UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("restore imported time: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect restored import time update: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("restore imported time: %w", ErrNotFound)
	}
	return nil
}

func (s *Store) ListResources(ctx context.Context, kind string) ([]library.Resource, error) {
	query := `SELECT id, source_hash, kind, subtype, name, description, source_filename,
source_rel_path, source_size, imported_at, details_json FROM resources`
	args := []any{}
	if kind != "" {
		query += " WHERE kind = ?"
		args = append(args, kind)
	}
	query += " ORDER BY imported_at DESC, name COLLATE NOCASE"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}
	defer rows.Close()
	resources := make([]library.Resource, 0)
	for rows.Next() {
		resource, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resources: %w", err)
	}
	return resources, nil
}

func (s *Store) GetResource(ctx context.Context, id string) (library.Resource, error) {
	const query = `SELECT id, source_hash, kind, subtype, name, description, source_filename,
source_rel_path, source_size, imported_at, details_json FROM resources WHERE id = ?`
	resource, err := scanResource(s.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return library.Resource{}, ErrNotFound
	}
	return resource, err
}

func (s *Store) GetResourceByHash(ctx context.Context, hash string) (library.Resource, error) {
	const query = `SELECT id, source_hash, kind, subtype, name, description, source_filename,
source_rel_path, source_size, imported_at, details_json FROM resources WHERE source_hash = ?`
	resource, err := scanResource(s.db.QueryRowContext(ctx, query, hash))
	if errors.Is(err, sql.ErrNoRows) {
		return library.Resource{}, ErrNotFound
	}
	return resource, err
}

func (s *Store) DeleteResource(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM resources WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete resource: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted resources: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) List(ctx context.Context) ([]library.Character, error) {
	const query = `SELECT id, source_hash, name, creator, spec, spec_version, tags_json,
has_worldbook, has_regex, has_extensions, has_interactive, source_format,
source_is_image, source_filename, source_rel_path, source_size, imported_at
 , manifest_json
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
 , manifest_json
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
 , manifest_json
FROM characters WHERE source_hash = ?`
	character, err := scanCharacter(s.db.QueryRowContext(ctx, query, hash))
	if errors.Is(err, sql.ErrNoRows) {
		return library.Character{}, ErrNotFound
	}
	return character, err
}

func (s *Store) UpdateParsed(ctx context.Context, character library.Character) error {
	tags, err := json.Marshal(character.Tags)
	if err != nil {
		return fmt.Errorf("encode tags: %w", err)
	}
	contentManifest, err := json.Marshal(character.Manifest)
	if err != nil {
		return fmt.Errorf("encode content manifest: %w", err)
	}
	const query = `UPDATE characters SET
name = ?, creator = ?, spec = ?, spec_version = ?, tags_json = ?,
has_worldbook = ?, has_regex = ?, has_extensions = ?, has_interactive = ?,
source_format = ?, source_is_image = ?, manifest_json = ?
WHERE id = ?`
	result, err := s.db.ExecContext(ctx, query,
		character.Name, character.Creator, character.Spec, character.SpecVersion, string(tags),
		character.HasWorldbook, character.HasRegex, character.HasExtensions, character.HasInteractive,
		character.SourceFormat, character.SourceIsImage, string(contentManifest), character.ID,
	)
	if err != nil {
		return fmt.Errorf("update parsed character content: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated characters: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
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

func scanResource(row scanner) (library.Resource, error) {
	var resource library.Resource
	var imported, detailsJSON string
	err := row.Scan(
		&resource.ID, &resource.SourceHash, &resource.Kind, &resource.Subtype,
		&resource.Name, &resource.Description, &resource.SourceFilename,
		&resource.SourceRelPath, &resource.SourceSize, &imported, &detailsJSON,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Resource{}, err
		}
		return library.Resource{}, fmt.Errorf("scan resource: %w", err)
	}
	resource.ImportedAt, err = time.Parse(time.RFC3339Nano, imported)
	if err != nil {
		return library.Resource{}, fmt.Errorf("decode resource import time: %w", err)
	}
	var details struct {
		Worldbook json.RawMessage `json:"worldbook"`
		Preset    json.RawMessage `json:"preset"`
	}
	if err := json.Unmarshal([]byte(detailsJSON), &details); err != nil {
		return library.Resource{}, fmt.Errorf("decode resource details: %w", err)
	}
	if len(details.Worldbook) > 0 && string(details.Worldbook) != "null" {
		if err := json.Unmarshal(details.Worldbook, &resource.Worldbook); err != nil {
			return library.Resource{}, fmt.Errorf("decode worldbook details: %w", err)
		}
	}
	if len(details.Preset) > 0 && string(details.Preset) != "null" {
		if err := json.Unmarshal(details.Preset, &resource.Preset); err != nil {
			return library.Resource{}, fmt.Errorf("decode preset details: %w", err)
		}
	}
	return resource, nil
}

func scanCharacter(row scanner) (library.Character, error) {
	var character library.Character
	var tags, imported, contentManifest string
	var worldbook, regex, extensions, interactive, image bool
	err := row.Scan(
		&character.ID, &character.SourceHash, &character.Name, &character.Creator,
		&character.Spec, &character.SpecVersion, &tags, &worldbook, &regex,
		&extensions, &interactive, &character.SourceFormat, &image,
		&character.SourceFilename, &character.SourceRelPath, &character.SourceSize, &imported, &contentManifest,
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
	if err := json.Unmarshal([]byte(contentManifest), &character.Manifest); err != nil {
		return library.Character{}, fmt.Errorf("decode content manifest: %w", err)
	}
	return character, nil
}

func IsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}
