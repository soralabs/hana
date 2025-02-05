package guardrails

import (
	"github.com/soralabs/zen/db"
	"github.com/soralabs/zen/manager"
)

// ManagerID for the guardrails manager
const GuardrailsManagerID manager.ManagerID = "guardrails"

const FragmentTableGuardrails db.FragmentTable = "guardrails"
