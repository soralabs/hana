package guardrails

import (
	"github.com/soralabs/zen/db"
	"github.com/soralabs/zen/manager"
	"github.com/soralabs/zen/state"
)

// ManagerID for the guardrails manager
const GuardrailsManagerID manager.ManagerID = "guardrails"

const FragmentTableGuardrails db.FragmentTable = "guardrails"

const GuardrailsResultKey state.StateDataKey = "guardrails_result"
