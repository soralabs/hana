package sora_manager

import (
	"github.com/soralabs/zen/cache"
	"github.com/soralabs/zen/manager"
	"github.com/soralabs/zen/options"
)

// SoraManager handles Sora-specific behavior and responses, such as information about Sora
type SoraManager struct {
	*manager.BaseManager
	options.RequiredFields

	cache *cache.Cache
}
