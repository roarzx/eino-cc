package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	RepoRoot string `yaml:"repo_root"`
	Model    struct {
		Provider string `yaml:"provider"`
		Model    string `yaml:"model"`
		APIKey   string `yaml:"api_key"`
	} `yaml:"model"`
	Agent struct {
		MaxIterations int  `yaml:"max_iterations"`
		DryRun        bool `yaml:"dry_run"`
	} `yaml:"agent"`
	Commands struct {
		Test string `yaml:"test"`
		Lint string `yaml:"lint"`
	} `yaml:"commands"`
}

func Load(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = "configs/default.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.RepoRoot == "." {
		cfg.RepoRoot, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	} else {
		cfg.RepoRoot, err = filepath.Abs(cfg.RepoRoot)
		if err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}