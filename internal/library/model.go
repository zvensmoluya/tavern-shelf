package library

import (
	"time"

	"github.com/zvensmoluya/tavern-shelf/internal/manifest"
)

type Character struct {
	CoverHash      string           `json:"coverHash,omitempty"`
	ContentHash    string           `json:"contentHash,omitempty"`
	IdentityKey    string           `json:"-"`
	GroupID        string           `json:"groupId"`
	GroupReason    string           `json:"groupReason"`
	ID             string           `json:"id"`
	SourceHash     string           `json:"sourceHash,omitempty"`
	Name           string           `json:"name"`
	Creator        string           `json:"creator,omitempty"`
	Spec           string           `json:"spec,omitempty"`
	SpecVersion    string           `json:"specVersion,omitempty"`
	Tags           []string         `json:"tags"`
	HasWorldbook   bool             `json:"hasWorldbook"`
	HasRegex       bool             `json:"hasRegex"`
	HasExtensions  bool             `json:"hasExtensions"`
	HasInteractive bool             `json:"hasInteractive"`
	SourceFormat   string           `json:"sourceFormat"`
	SourceIsImage  bool             `json:"sourceIsImage"`
	SourceFilename string           `json:"sourceFilename"`
	SourceRelPath  string           `json:"-"`
	SourceSize     int64            `json:"sourceSize"`
	ImportedAt     time.Time        `json:"importedAt"`
	AvatarURL      string           `json:"avatarUrl,omitempty"`
	SourceURL      string           `json:"sourceUrl"`
	Manifest       manifest.Content `json:"manifest"`
	Favorite       bool             `json:"favorite"`
	Note           string           `json:"note,omitempty"`
	CollectionIDs  []string         `json:"collectionIds"`
}

type CreateCharacter struct {
	Character
}

type CharacterOrganization struct {
	Favorite      bool     `json:"favorite"`
	Note          string   `json:"note"`
	CollectionIDs []string `json:"collectionIds"`
}

type Collection struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	CharacterCount int       `json:"characterCount"`
	CreatedAt      time.Time `json:"createdAt"`
}
