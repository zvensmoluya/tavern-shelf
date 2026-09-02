package library

import (
	"time"

	"github.com/zvensmoluya/tavern-shelf/internal/manifest"
)

const (
	ResourceWorldbook = "worldbook"
	ResourcePreset    = "preset"
)

type Resource struct {
	ID             string                  `json:"id"`
	SourceHash     string                  `json:"sourceHash"`
	Kind           string                  `json:"kind"`
	Subtype        string                  `json:"subtype,omitempty"`
	Name           string                  `json:"name"`
	Description    string                  `json:"description,omitempty"`
	SourceFilename string                  `json:"sourceFilename"`
	SourceRelPath  string                  `json:"-"`
	SourceSize     int64                   `json:"sourceSize"`
	ImportedAt     time.Time               `json:"importedAt"`
	SourceURL      string                  `json:"sourceUrl"`
	Worldbook      *manifest.CharacterBook `json:"worldbook,omitempty"`
	Preset         *manifest.Preset        `json:"preset,omitempty"`
}
