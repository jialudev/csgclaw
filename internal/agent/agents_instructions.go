package agent

import (
	"bytes"
	_ "embed"
	"strings"
	"text/template"
)

const (
	agentsInstructionsBlockStart = "<!-- BEGIN CSGCLAW-INSTRUCTIONS (auto-generated; do not edit) -->"
	agentsInstructionsBlockEnd   = "<!-- END CSGCLAW-INSTRUCTIONS -->"
)

//go:embed embed/agents_instructions.md.tmpl
var agentsInstructionsBlockTemplate string

var parsedAgentsInstructionsBlockTemplate = template.Must(
	template.New("agents_instructions_block").Parse(agentsInstructionsBlockTemplate),
)

func AgentsInstructionsBlockMarkers() (start string, end string) {
	return agentsInstructionsBlockStart, agentsInstructionsBlockEnd
}

func RenderAgentsInstructionsBlock(instructions string) string {
	instructions = strings.TrimSpace(instructions)
	data := struct {
		StartMarker     string
		EndMarker       string
		Instructions    string
		HasInstructions bool
	}{
		StartMarker:     agentsInstructionsBlockStart,
		EndMarker:       agentsInstructionsBlockEnd,
		Instructions:    instructions,
		HasInstructions: instructions != "",
	}
	var b bytes.Buffer
	if err := parsedAgentsInstructionsBlockTemplate.Execute(&b, data); err != nil {
		panic("render agents instructions block: " + err.Error())
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}
