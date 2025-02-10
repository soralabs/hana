package sora_manager

import (
	"time"

	"github.com/soralabs/zen/cache"
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
		cache: cache.New(cache.Config{
			MaxSize:       1000,
			TTL:           1 * time.Minute,
			CleanupPeriod: 1 * time.Minute,
		}),
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
	soraInfo := state.StateData{
		Key: SoraInformation,
		Value: `Sora Labs is a pioneering AI research and development company focused on bridging the Solana ecosystem with cutting-edge artificial intelligence. 
The company's focus is on empowering developers and projects in the Solana ecosystem with advanced AI capabilities through two flagship products: the Zen framework and Hana, an AI agent.

Zen is a sophisticated AI agent framework built in Go, designed to create, deploy, and manage intelligent agents at scale. 
It features a plugin-based architecture with a powerful manager system for extending functionality, including specialized components like the Insight Manager for conversation analysis and the Personality Manager for dynamic response behavior. 
The framework provides robust state management with a centralized system supporting cross-manager communication and custom data injection. 
Zen's LLM integration layer offers seamless support for multiple providers, with built-in OpenAI compatibility and an extensible interface for custom LLMs. 
The platform-agnostic core enables deployment across various platforms, with native support for CLI and Twitter interactions. 
Data persistence is handled through a flexible storage layer utilizing PostgreSQL with pgvector for semantic search capabilities. 
The framework includes a comprehensive toolkit system for custom function integration, complete with state-aware execution and automatic response handling.

Hana is Sora Labs' flagship AI agent, built on the Zen framework. 
As the primary AI representative, she embodies the technical sophistication of Zen while maintaining a uniquely engaging personality. 
Hana serves as both a practical demonstration of Zen's capabilities and a bridge between Sora Labs and the community, showcasing how AI agents can provide meaningful interactions while handling complex tasks in the Solana ecosystem.`,
	}

	returnData := []state.StateData{soraInfo}

	soraTokenData, err := s.getSoraTokenData()
	if err != nil {
		returnData = append(returnData, state.StateData{
			Key:   SoraTokenData,
			Value: soraTokenData,
		})
	}

	return returnData, nil
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
