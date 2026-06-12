package agent

import (
	"strings"
	"testing"
)

func TestAgentsInstructionsBlockMarkers(t *testing.T) {
	start, end := AgentsInstructionsBlockMarkers()
	if start != agentsInstructionsBlockStart {
		t.Fatalf("AgentsInstructionsBlockMarkers() start = %q, want %q", start, agentsInstructionsBlockStart)
	}
	if end != agentsInstructionsBlockEnd {
		t.Fatalf("AgentsInstructionsBlockMarkers() end = %q, want %q", end, agentsInstructionsBlockEnd)
	}
}

func TestRenderAgentsInstructionsBlockIncludesEmbeddedRules(t *testing.T) {
	got := RenderAgentsInstructionsBlock("")
	for _, want := range []string{
		"### Scope",
		"### Workflow",
		"### Room And Participant Rules",
		"### Worker Notification Rules",
		"### Operating Rules",
		"`mention_only`",
		"`agent-teams`",
		"`manager`",
		"`u-manager`",
		"`participant list`",
		"`member list`",
		"`message list`",
		"`<at user_id=\"...\">`",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("RenderAgentsInstructionsBlock() missing excerpt %q in %q", want, got)
		}
	}
	if strings.Contains(got, "basics/SKILL.md") {
		t.Fatalf("RenderAgentsInstructionsBlock() = %q, want embedded rules instead of template reference", got)
	}
	if strings.Contains(got, "```") {
		t.Fatalf("RenderAgentsInstructionsBlock() = %q, want summarized rules instead of full command examples", got)
	}
}

func TestAgentsInstructionsTemplateUsesMarkers(t *testing.T) {
	got := RenderAgentsInstructionsBlock("")
	if !strings.Contains(got, agentsInstructionsBlockStart) {
		t.Fatalf("RenderAgentsInstructionsBlock() = %q, want start marker from Go constant", got)
	}
	if !strings.Contains(got, agentsInstructionsBlockEnd) {
		t.Fatalf("RenderAgentsInstructionsBlock() = %q, want end marker from Go constant", got)
	}
}

func TestRenderAgentsInstructionsBlockWithoutInstructions(t *testing.T) {
	got := RenderAgentsInstructionsBlock("  ")

	startIdx := strings.Index(got, agentsInstructionsBlockStart)
	rulesIdx := strings.Index(got, "# CSGClaw Rules")
	if startIdx < 0 || rulesIdx < 0 || startIdx >= rulesIdx {
		t.Fatalf("RenderAgentsInstructionsBlock() = %q, want rules heading after start marker", got)
	}
	if strings.Contains(got, "# Agent Instructions") {
		t.Fatalf("RenderAgentsInstructionsBlock() = %q, want no agent instructions section", got)
	}
	if !strings.Contains(got, "# CSGClaw Rules\n\n### Scope") {
		t.Fatalf("RenderAgentsInstructionsBlock() = %q, want embedded rules section", got)
	}
	if !strings.HasSuffix(got, agentsInstructionsBlockEnd+"\n") {
		t.Fatalf("RenderAgentsInstructionsBlock() suffix = %q", got)
	}
}

func TestRenderAgentsInstructionsBlockWithInstructions(t *testing.T) {
	instructions := "Use Chinese for teammate-facing docs.\nKeep code identifiers unchanged."
	got := RenderAgentsInstructionsBlock(instructions)

	agentInstructionsIdx := strings.Index(got, "# Agent Instructions")
	if agentInstructionsIdx < 0 {
		t.Fatalf("RenderAgentsInstructionsBlock() = %q, want agent instructions section", got)
	}
	instructionsIdx := strings.Index(got, instructions)
	if instructionsIdx < 0 {
		t.Fatalf("RenderAgentsInstructionsBlock() = %q, want instructions body", got)
	}
	rulesIdx := strings.Index(got, "# CSGClaw Rules")
	if rulesIdx < 0 {
		t.Fatalf("RenderAgentsInstructionsBlock() = %q, want CSGClaw rules section", got)
	}
	if !(agentInstructionsIdx < instructionsIdx && instructionsIdx < rulesIdx) {
		t.Fatalf("RenderAgentsInstructionsBlock() = %q, want instructions section before rules", got)
	}
	if strings.Count(got, agentsInstructionsBlockStart) != 1 {
		t.Fatalf("RenderAgentsInstructionsBlock() start marker count = %d, want 1", strings.Count(got, agentsInstructionsBlockStart))
	}
	if strings.Count(got, agentsInstructionsBlockEnd) != 1 {
		t.Fatalf("RenderAgentsInstructionsBlock() end marker count = %d, want 1", strings.Count(got, agentsInstructionsBlockEnd))
	}
}
