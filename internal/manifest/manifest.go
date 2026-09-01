package manifest

const CurrentSchemaVersion = 1

// Content is a deterministic, display-oriented projection of a character
// card. It never replaces the original source and can always be rebuilt.
type Content struct {
	SchemaVersion int            `json:"schemaVersion"`
	Character     Character      `json:"character"`
	Greetings     Greetings      `json:"greetings"`
	CharacterBook *CharacterBook `json:"characterBook,omitempty"`
	RegexScripts  []RegexScript  `json:"regexScripts"`
	Extensions    []Extension    `json:"extensions"`
	Assets        []Asset        `json:"assets"`
	Sources       []string       `json:"sources"`
	Interaction   Interaction    `json:"interaction"`
	CreationDate  int64          `json:"creationDate,omitempty"`
	ModifiedDate  int64          `json:"modifiedDate,omitempty"`
}

type Character struct {
	Name                     string            `json:"name"`
	Nickname                 string            `json:"nickname,omitempty"`
	Creator                  string            `json:"creator,omitempty"`
	CharacterVersion         string            `json:"characterVersion,omitempty"`
	Tags                     []string          `json:"tags"`
	Description              string            `json:"description,omitempty"`
	Personality              string            `json:"personality,omitempty"`
	Scenario                 string            `json:"scenario,omitempty"`
	MessageExample           string            `json:"messageExample,omitempty"`
	CreatorNotes             string            `json:"creatorNotes,omitempty"`
	CreatorNotesMultilingual map[string]string `json:"creatorNotesMultilingual,omitempty"`
	SystemPrompt             string            `json:"systemPrompt,omitempty"`
	PostHistoryInstructions  string            `json:"postHistoryInstructions,omitempty"`
}

type Greetings struct {
	FirstMessage   string   `json:"firstMessage,omitempty"`
	Alternate      []string `json:"alternate"`
	GroupOnly      []string `json:"groupOnly"`
	TotalCount     int      `json:"totalCount"`
	AlternateCount int      `json:"alternateCount"`
	GroupOnlyCount int      `json:"groupOnlyCount"`
}

type CharacterBook struct {
	Name              string               `json:"name,omitempty"`
	Description       string               `json:"description,omitempty"`
	EntryCount        int                  `json:"entryCount"`
	EnabledEntryCount int                  `json:"enabledEntryCount"`
	ScanDepth         *int                 `json:"scanDepth,omitempty"`
	TokenBudget       *int                 `json:"tokenBudget,omitempty"`
	RecursiveScanning *bool                `json:"recursiveScanning,omitempty"`
	Entries           []CharacterBookEntry `json:"entries"`
}

type CharacterBookEntry struct {
	Name           string   `json:"name"`
	Comment        string   `json:"comment,omitempty"`
	Keys           []string `json:"keys"`
	SecondaryKeys  []string `json:"secondaryKeys"`
	Content        string   `json:"content,omitempty"`
	Enabled        bool     `json:"enabled"`
	Constant       bool     `json:"constant"`
	Selective      bool     `json:"selective"`
	UseRegex       bool     `json:"useRegex"`
	CaseSensitive  bool     `json:"caseSensitive"`
	InsertionOrder int      `json:"insertionOrder"`
}

type RegexScript struct {
	Name          string `json:"name"`
	FindRegex     string `json:"findRegex,omitempty"`
	ReplaceString string `json:"replaceString,omitempty"`
	Placement     []int  `json:"placement"`
	Disabled      bool   `json:"disabled"`
	MarkdownOnly  bool   `json:"markdownOnly"`
	PromptOnly    bool   `json:"promptOnly"`
	RunOnEdit     bool   `json:"runOnEdit"`
	MinDepth      *int   `json:"minDepth,omitempty"`
	MaxDepth      *int   `json:"maxDepth,omitempty"`
}

type Extension struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type Asset struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Ext     string `json:"ext,omitempty"`
	URIKind string `json:"uriKind,omitempty"`
}

type Interaction struct {
	HasHTML                 bool     `json:"hasHtml"`
	HasJavaScript           bool     `json:"hasJavaScript"`
	HasInteractiveExtension bool     `json:"hasInteractiveExtension"`
	Markers                 []string `json:"markers"`
}

type Preset struct {
	Type        string        `json:"type"`
	FieldCount  int           `json:"fieldCount"`
	PromptCount int           `json:"promptCount,omitempty"`
	Fields      []PresetField `json:"fields"`
	TextBlocks  []PresetField `json:"textBlocks"`
}

type PresetField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}

func (c Content) Empty() bool {
	return c.SchemaVersion < CurrentSchemaVersion || c.Character.Name == ""
}
