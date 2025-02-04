package sora_manager

import (
	"github.com/soralabs/zen/db"
	"github.com/soralabs/zen/manager"
	"github.com/soralabs/zen/state"
)

const (
	SoraManagerID manager.ManagerID = "sora"
)

const (
	FragmentTableSora db.FragmentTable = "sora"
)

const (
	SoraInformation state.StateDataKey = "sora_information"
)
