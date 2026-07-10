package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Crypto   CryptoConfig   `yaml:"crypto"`
	SSH      SSHConfig      `yaml:"ssh"`
	Session  SessionConfig  `yaml:"session"`
	AI       AIConfig       `yaml:"ai"`
	Sync     SyncConfig     `yaml:"sync"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type JWTConfig struct {
	Secret string `yaml:"secret"`
	Expire int    `yaml:"expire"`
}

type CryptoConfig struct {
	Key string `yaml:"key"`
}

type SSHConfig struct {
	KeepaliveInterval int  `yaml:"keepalive_interval"`
	KeepaliveCount    int  `yaml:"keepalive_count"`
	LegacyAlgorithms  bool `yaml:"legacy_algorithms"`
}

type SessionConfig struct {
	RecordDir string `yaml:"record_dir"`
}

type AIConfig struct {
	Provider        string `yaml:"provider"`
	APIKey          string `yaml:"api_key"`
	BaseURL         string `yaml:"base_url"`
	Model           string `yaml:"model"`
	MaxContextLines int    `yaml:"max_context_lines"`
}

type SyncConfig struct {
	Enabled  bool                   `yaml:"enabled"`
	Provider string                 `yaml:"provider"`
	Interval int                    `yaml:"interval"`
	Config   map[string]interface{} `yaml:"config"`
}

var Global *Config

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Defaults
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.JWT.Secret == "" {
		cfg.JWT.Secret = "default-secret-change-me"
	}
	if cfg.JWT.Expire == 0 {
		cfg.JWT.Expire = 72
	}

	// Auto-generate crypto key if empty
	if cfg.Crypto.Key == "" {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("generate crypto key: %w", err)
		}
		cfg.Crypto.Key = hex.EncodeToString(key)
		// Save back to file
		saveConfig(path, cfg)
	}

	// Ensure data directories exist
	if cfg.Database.DSN != "" {
		dir := filepath.Dir(cfg.Database.DSN)
		os.MkdirAll(dir, 0755)
	}
	if cfg.Session.RecordDir != "" {
		os.MkdirAll(cfg.Session.RecordDir, 0755)
	}

	Global = cfg
	return cfg, nil
}

func saveConfig(path string, cfg *Config) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return
	}
	os.WriteFile(path, data, 0600)
}
