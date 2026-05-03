package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Server string `json:"server"`
	Token  string `json:"token"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".crawlerv2.json")
}

func loadConfig() Config {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return Config{}
	}
	var cfg Config
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

func saveConfig(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0600)
}
