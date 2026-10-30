// Package skill holds the agent skill, built into the binary so the page can hand it to an agent.
package skill

import _ "embed"

// Text is SKILL.md.
//
//go:embed shipwake/SKILL.md
var Text string
