package storage

import (
	"encoding/json"
	"fmt"
	"os"
)

// StoreConfig describes one storage backend in the config file.
type StoreConfig struct {
	Type string    `json:"type"` // "local" or "s3"
	Path string    `json:"path"` // local only
	S3   *S3Config `json:"s3"`   // s3 only
}

// Config is the top-level storage configuration file.
type Config struct {
	// Stores maps store ID → config.
	Stores map[string]StoreConfig `json:"stores"`

	// DefaultStore is used when no domain-specific mapping exists.
	DefaultStore string `json:"default_store"`

	// DomainStores maps domain ID (as string) → store ID.
	// e.g. {"1": "s3-lt1", "2": "s3-lt10"}
	DomainStores map[string]string `json:"domain_stores"`
}

// LoadConfig reads a JSON config file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("storage config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("storage config parse: %w", err)
	}
	if cfg.DefaultStore == "" && len(cfg.Stores) > 0 {
		for id := range cfg.Stores {
			cfg.DefaultStore = id
			break
		}
	}
	return &cfg, nil
}

// DefaultLocalConfig returns a minimal config using local storage at the given path.
func DefaultLocalConfig(filesDir string) *Config {
	return &Config{
		Stores:       map[string]StoreConfig{"local": {Type: "local", Path: filesDir}},
		DefaultStore: "local",
		DomainStores: map[string]string{},
	}
}
