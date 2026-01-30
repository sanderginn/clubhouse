package services

import (
	"context"
	"database/sql"
	"errors"
	"sync"
)

// Config holds application configuration that can be toggled at runtime
type Config struct {
	LinkMetadataEnabled bool   `json:"linkMetadataEnabled"`
	MFARequired         bool   `json:"mfaRequired"`
	DisplayTimezone     string `json:"displayTimezone"`
}

// ConfigService provides thread-safe access to runtime configuration
type ConfigService struct {
	mu     sync.RWMutex
	config Config
	db     *sql.DB
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
				MFARequired:         false,
				DisplayTimezone:     "UTC",
			},
		}
	})
	return globalConfigService
}

// InitConfigService wires the config service to the database and loads persisted config.
func InitConfigService(ctx context.Context, db *sql.DB) error {
	service := GetConfigService()
	service.mu.Lock()
	service.db = db
	service.mu.Unlock()

	return service.loadFromDB(ctx)
}

// GetConfig returns a copy of the current configuration
func (s *ConfigService) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig updates the configuration with the provided values
func (s *ConfigService) UpdateConfig(ctx context.Context, linkMetadataEnabled *bool, mfaRequired *bool, displayTimezone *string) (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated := s.config
	if linkMetadataEnabled != nil {
		updated.LinkMetadataEnabled = *linkMetadataEnabled
	}
	if mfaRequired != nil {
		updated.MFARequired = *mfaRequired
	}
	if displayTimezone != nil {
		updated.DisplayTimezone = *displayTimezone
	}

	if s.db != nil {
		if ctx == nil {
			ctx = context.Background()
		}
		if err := s.persistConfig(ctx, updated); err != nil {
			return s.config, err
		}
	}

	s.config = updated
	return s.config, nil
}

// IsLinkMetadataEnabled returns whether link metadata fetching is enabled
func (s *ConfigService) IsLinkMetadataEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.LinkMetadataEnabled
}

// IsMFARequired returns whether MFA enrollment is required for all users.
func (s *ConfigService) IsMFARequired() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.MFARequired
}

// ResetConfigServiceForTests resets the config service to defaults and clears the database handle.
func ResetConfigServiceForTests() {
	service := GetConfigService()
	service.mu.Lock()
	defer service.mu.Unlock()
	service.db = nil
	service.config = Config{
		LinkMetadataEnabled: true,
		MFARequired:         false,
		DisplayTimezone:     "UTC",
	}
}

func (s *ConfigService) loadFromDB(ctx context.Context) error {
	s.mu.RLock()
	db := s.db
	defaults := s.config
	s.mu.RUnlock()
	if db == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var config Config
	err := db.QueryRowContext(ctx, `
		SELECT link_metadata_enabled, mfa_required, display_timezone
		FROM admin_config
		WHERE id = 1
	`).Scan(&config.LinkMetadataEnabled, &config.MFARequired, &config.DisplayTimezone)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if err := s.persistConfig(ctx, defaults); err != nil {
				return err
			}
			s.mu.Lock()
			s.config = defaults
			s.mu.Unlock()
			return nil
		}
		return err
	}
	if config.DisplayTimezone == "" {
		config.DisplayTimezone = "UTC"
	}

	s.mu.Lock()
	s.config = config
	s.mu.Unlock()
	return nil
}

func (s *ConfigService) persistConfig(ctx context.Context, config Config) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO admin_config (id, link_metadata_enabled, mfa_required, display_timezone)
		VALUES (1, $1, $2, $3)
		ON CONFLICT (id) DO UPDATE
		SET link_metadata_enabled = EXCLUDED.link_metadata_enabled,
			mfa_required = EXCLUDED.mfa_required,
			display_timezone = EXCLUDED.display_timezone,
			updated_at = now()
	`, config.LinkMetadataEnabled, config.MFARequired, config.DisplayTimezone)
	return err
}
