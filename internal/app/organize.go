package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/openai/tavern-shelf/internal/library"
	"github.com/openai/tavern-shelf/internal/store"
)

const (
	maxCollectionNameLength = 80
	maxCharacterNoteLength  = 20_000
)

func (a *App) Collections(ctx context.Context) ([]library.Collection, error) {
	return a.Store.ListCollections(ctx)
}

func (a *App) CreateCollection(ctx context.Context, name string) (library.Collection, error) {
	name, err := validCollectionName(name)
	if err != nil {
		return library.Collection{}, err
	}
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return library.Collection{}, fmt.Errorf("create collection ID: %w", err)
	}
	collection := library.Collection{ID: hex.EncodeToString(random), Name: name, CreatedAt: time.Now().UTC()}
	if err := a.Store.CreateCollection(ctx, collection); err != nil {
		if store.IsUniqueViolation(err) {
			return library.Collection{}, errors.New("a collection with this name already exists")
		}
		return library.Collection{}, err
	}
	return collection, nil
}

func (a *App) RenameCollection(ctx context.Context, id, name string) error {
	name, err := validCollectionName(name)
	if err != nil {
		return err
	}
	if err := a.Store.RenameCollection(ctx, id, name); err != nil {
		if store.IsUniqueViolation(err) {
			return errors.New("a collection with this name already exists")
		}
		return err
	}
	return nil
}

func (a *App) DeleteCollection(ctx context.Context, id string) error {
	return a.Store.DeleteCollection(ctx, id)
}

func (a *App) OrganizeCharacter(ctx context.Context, id string, organization library.CharacterOrganization) error {
	if utf8.RuneCountInString(organization.Note) > maxCharacterNoteLength {
		return fmt.Errorf("private note exceeds %d characters", maxCharacterNoteLength)
	}
	organization.Note = strings.TrimSpace(organization.Note)
	return a.Store.SaveCharacterOrganization(ctx, id, organization)
}

func validCollectionName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("collection name is required")
	}
	if utf8.RuneCountInString(name) > maxCollectionNameLength {
		return "", fmt.Errorf("collection name exceeds %d characters", maxCollectionNameLength)
	}
	if strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", errors.New("collection name cannot contain control characters")
	}
	return name, nil
}
