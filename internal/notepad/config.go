package notepad

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Auth   AuthConfig   `yaml:"auth"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
}

type AuthConfig struct {
	Password          string `yaml:"password"`
	JWTSecret         string `yaml:"jwt_secret"`
	SessionExpiresIn  string `yaml:"session_expires_in"`
	RememberExpiresIn string `yaml:"remember_expires_in"`
}

func LoadConfig(baseDir string) (*Config, error) {
	configPath := filepath.Join(baseDir, "config.yaml")

	cfg := &Config{}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		secret, err := generateSecret()
		if err != nil {
			return nil, fmt.Errorf("generate jwt_secret: %w", err)
		}
		cfg.Server.Port = defaultPort
		cfg.Auth.Password = defaultPassword
		cfg.Auth.JWTSecret = secret
		cfg.Auth.SessionExpiresIn = defaultSessionExpiresIn
		cfg.Auth.RememberExpiresIn = defaultRememberExpiresIn

		if err := writeConfig(configPath, cfg); err != nil {
			return nil, fmt.Errorf("write config.yaml: %w", err)
		}
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config.yaml: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config.yaml: %w", err)
	}

	changed := false

	if cfg.Server.Port == "" {
		cfg.Server.Port = defaultPort
		changed = true
	}
	if cfg.Auth.Password == "" {
		cfg.Auth.Password = defaultPassword
		changed = true
	}
	if cfg.Auth.SessionExpiresIn == "" {
		cfg.Auth.SessionExpiresIn = defaultSessionExpiresIn
		changed = true
	}
	if cfg.Auth.RememberExpiresIn == "" {
		cfg.Auth.RememberExpiresIn = defaultRememberExpiresIn
		changed = true
	}
	if cfg.Auth.JWTSecret == "" {
		secret, err := generateSecret()
		if err != nil {
			return nil, fmt.Errorf("generate jwt_secret: %w", err)
		}
		cfg.Auth.JWTSecret = secret
		changed = true
	}

	if changed {
		if err := writeConfig(configPath, cfg); err != nil {
			return nil, fmt.Errorf("write config.yaml: %w", err)
		}
	}

	return cfg, nil
}

func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func writeConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}