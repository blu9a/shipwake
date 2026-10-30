package model

// Config is a project's optional .shipwake.yaml. Every field has a default, so a project
// with no config works: its directory name, a trunk of main, English only.
type Config struct {
	Name      string            `json:"name,omitempty" yaml:"name"`
	Trunk     string            `json:"trunk,omitempty" yaml:"trunk"`
	Languages []string          `json:"languages,omitempty" yaml:"languages"`       // the first is required for release notes
	Targets   map[string]Names  `json:"targets,omitempty" yaml:"targets"`           // display names for targets, per language
	Text      map[string]string `json:"translations,omitempty" yaml:"translations"` // language → file with project text translations
	// Measurements by ID or pattern: which way is better, and thresholds as absolute numbers or
	// "+15%" / "-10%" relative to the baseline. Metrics agreed on as requirements are .metric files
	// instead.
	Metrics map[string]struct {
		Direction string `json:"direction,omitempty" yaml:"direction"` // higher or lower is better
		Warn      any    `json:"warn,omitempty" yaml:"warn"`
		Block     any    `json:"block,omitempty" yaml:"block"`
	} `json:"metrics,omitempty" yaml:"metrics"`
	// Run kinds: an icon (test, gauge, bolt, timer, shield, chart) and names in other languages, such as
	// {icon: gauge, zh: 评测}.
	Kinds map[string]map[string]string `json:"kinds,omitempty" yaml:"kinds"`
	// The changelog: a path (default CHANGELOG.md, and CHANGELOG.<lang>.md for each other language),
	// a map from language to path, or "none" to keep release notes in tags only.
	Changelog any `json:"changelog,omitempty" yaml:"changelog"`
	// Where runs are stored when no --results is given: a directory, or "git:<branch>" (default git:shipwake-results).
	Results string `json:"results,omitempty" yaml:"results"`
}
