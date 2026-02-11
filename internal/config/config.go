package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	RepoRoot string `yaml:"repo_root"`
	Model    Model  `yaml:"model"`
	Agent    Agent  `yaml:"agent"`
	Commands Commands `yaml:"commands"`
}

type Model struct {
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model"`
	BaseURL   string `yaml:"base_url"`
	APIKeyEnv string `yaml:"api_key_env"`
}

type Agent struct {
	MaxIterations int `yaml:"max_iterations"`
}

type Commands struct {
	Test string `yaml:"test"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		return nil, errors.New("config path is empty")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config yaml: %w", err)
	}
	if cfg.RepoRoot == "" {
		cfg.RepoRoot = "."
	}
	if cfg.Agent.MaxIterations <= 0 {
		cfg.Agent.MaxIterations = 2
	}
	if cfg.Commands.Test == "" {
		cfg.Commands.Test = "go test ./..."
	}
	return &cfg, nil
}

func (c *Config) AbsRepoRoot() (string, error) {
	if c.RepoRoot == "" {
		return "", errors.New("repo_root is empty")
	}
	abs, err := filepath.Abs(c.RepoRoot)
	if err != nil {
		return "", fmt.Errorf("abs repo_root: %w", err)
	}
	return abs, nil
}

