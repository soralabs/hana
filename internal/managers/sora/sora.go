package sora_manager

import (
	"github.com/soralabs/zen/db"
	"github.com/soralabs/zen/manager"
	"github.com/soralabs/zen/options"
	"github.com/soralabs/zen/state"
)

func NewSoraManager(
	baseOpts []options.Option[manager.BaseManager],
) (*SoraManager, error) {
	base, err := manager.NewBaseManager(baseOpts...)
	if err != nil {
		return nil, err
	}

	pm := &SoraManager{
		BaseManager: base,
	}

	if err := options.ApplyOptions(pm); err != nil {
		return nil, err
	}

	return pm, nil
}

func (s *SoraManager) GetID() manager.ManagerID {
	return SoraManagerID
}

// Process analyzes messages for Sora-relevant information
// Currently unimplemented as Sora is statically configured
func (s *SoraManager) Process(currentState *state.State) error {
	// Implement
	return nil
}

// PostProcess performs Sora-driven actions
// Currently unimplemented as Sora is statically configured
func (s *SoraManager) PostProcess(currentState *state.State) error {
	// Implement
	return nil
}

// Context formats and returns the current Sora configuration
// This is used in the prompt template to guide agent behavior
func (s *SoraManager) Context(currentState *state.State) ([]state.StateData, error) {
	// Implement
	return []state.StateData{}, nil
}

// Store persists a message fragment to storage
// Currently unimplemented as Sora configuration is static
func (s *SoraManager) Store(fragment *db.Fragment) error {
	// Implement
	return nil
}

// StartBackgroundProcesses initializes any background tasks
// Currently unimplemented as Sora configuration is static
func (s *SoraManager) StartBackgroundProcesses() {
	// Implement
}

// StopBackgroundProcesses cleanly shuts down any background tasks
// Currently unimplemented as no background processes exist
func (s *SoraManager) StopBackgroundProcesses() {
	// Implement
}
