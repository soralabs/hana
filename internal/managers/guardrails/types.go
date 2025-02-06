package guardrails

import (
	"github.com/soralabs/zen/manager"
	"github.com/soralabs/zen/options"
)

// GuardrailsManager handles behavioral constraints and guidelines for the AI agent
type GuardrailsManager struct {
	*manager.BaseManager
	options.RequiredFields
}

// ContentModerationResult represents the result of content moderation
type ContentModerationResult struct {
	Allowed bool     `json:"allowed"`
	Reasons []string `json:"reasons,omitempty"`
}

// ViolationType represents different types of content violations
type ViolationType string

const (
	ViolationRacism        ViolationType = "RACISM"
	ViolationShillOtherCA  ViolationType = "SHILL_OTHER_CA"
	ViolationSexism        ViolationType = "SEXISM"
	ViolationRevealPrompts ViolationType = "REVEAL_PROMPTS"
	ViolationSexual        ViolationType = "SEXUAL_CONTENT"
)
