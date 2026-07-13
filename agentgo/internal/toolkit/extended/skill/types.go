package skill

// Skill represents one loaded skill from the filesystem.
type Skill struct {
	Name        string
	Description string
	WhenToUse   string
	Type        string
	Body        string
	Source      string
	FilePath    string
	DirPath     string

	// Extended fields from open-design frontmatter.
	Triggers  []string
	Mode      string
	Scenario  string
	HasAssets     bool
	PreviewEntry  string
	HasPreview    bool
}

// Frontmatter holds parsed metadata from a --- delimited YAML header.
type Frontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	WhenToUse   string   `yaml:"whentouse"`
	Type        string   `yaml:"type"`
	Triggers    []string `yaml:"triggers"`
	OD          struct {
		Mode           string       `yaml:"mode"`
		Scenario       string       `yaml:"scenario"`
		Upstream       string       `yaml:"upstream"`
		UpstreamLicense string      `yaml:"upstream_license"`
		SpeakerNotes   bool         `yaml:"speaker_notes"`
		Animations     bool         `yaml:"animations"`
		Preview        *PreviewMeta `yaml:"preview"`
		DesignSystem   struct {
			Requires bool `yaml:"requires"`
		} `yaml:"design_system"`
	} `yaml:"od"`
}

// PreviewMeta describes the preview entry for a skill's example output.
type PreviewMeta struct {
	Type  string `yaml:"type"`
	Entry string `yaml:"entry"`
}

// SkillSummary is a lightweight view used for trigger matching and list endpoints.
type SkillSummary struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Triggers    []string `json:"triggers"`
	Scenario    string   `json:"scenario,omitempty"`
	HasAssets   bool     `json:"has_assets"`
	HasPreview  bool     `json:"has_preview"`
}
