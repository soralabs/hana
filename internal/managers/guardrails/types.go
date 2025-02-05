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
