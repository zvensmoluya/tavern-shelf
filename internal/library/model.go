package library

import (
	"time"

	"github.com/openai/tavern-shelf/internal/manifest"
)

type Character struct {
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
}

type CreateCharacter struct {
	Character
}
