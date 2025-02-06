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
