package sora_manager

import (
	"github.com/soralabs/zen/db"
	"github.com/soralabs/zen/manager"
	"github.com/soralabs/zen/options"
)

const (
	SoraManagerID     manager.ManagerID = "sora"
	FragmentTableSora db.FragmentTable  = "sora"
)

// SoraManager handles Sora-specific behavior and responses, such as information about Sora
type SoraManager struct {
	*manager.BaseManager
	options.RequiredFields
}
