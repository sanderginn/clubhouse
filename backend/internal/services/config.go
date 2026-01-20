package services

import (
	"sync"
)

// Config holds application configuration that can be toggled at runtime
type Config struct {
	LinkMetadataEnabled bool `json:"linkMetadataEnabled"`
}

// ConfigService provides thread-safe access to runtime configuration
type ConfigService struct {
	mu     sync.RWMutex
	config Config
}

// Global config service instance
var globalConfigService *ConfigService
var configOnce sync.Once

// GetConfigService returns the singleton config service instance
func GetConfigService() *ConfigService {
	configOnce.Do(func() {
		globalConfigService = &ConfigService{
			config: Config{
				LinkMetadataEnabled: true, // Enabled by default
			},
		}
	})
	return globalConfigService
}

// GetConfig returns a copy of the current configuration
func (s *ConfigService) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig updates the configuration with the provided values
func (s *ConfigService) UpdateConfig(linkMetadataEnabled *bool) Config {
	s.mu.Lock()
	defer s.mu.Unlock()

	if linkMetadataEnabled != nil {
		s.config.LinkMetadataEnabled = *linkMetadataEnabled
	}

	return s.config
}

// IsLinkMetadataEnabled returns whether link metadata fetching is enabled
func (s *ConfigService) IsLinkMetadataEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.LinkMetadataEnabled
}
