package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/heartleo/zlib/internal/zlib"
)

func BooxConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "zlib", "boox.json")
}

func LoadBooxConfig() (zlib.BooxConfig, error) {
	data, err := os.ReadFile(BooxConfigPath())
	if err != nil {
		return zlib.BooxConfig{}, err
	}
	var cfg zlib.BooxConfig
	return cfg, json.Unmarshal(data, &cfg)
}

func SaveBooxConfig(cfg zlib.BooxConfig) error {
	path := BooxConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
