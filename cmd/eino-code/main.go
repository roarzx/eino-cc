package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"eino-cc/internal/agent"
	"eino-cc/internal/config"
)

func main() {
	ctx := context.Background()

	configPath := flag.String("config", "configs/default.yaml", "")
	repoRoot := flag.String("repo-root", "", "")
	modelProvider := flag.String("model-provider", "", "")
	modelName := flag.String("model", "", "")
	baseURL := flag.String("base-url", "", "")
	apiKeyEnv := flag.String("api-key-env", "", "")
	maxIterations := flag.Int("max-iterations", 0, "")
	testCmd := flag.String("test-cmd", "", "")
	flag.Parse()

	goal := strings.TrimSpace(strings.Join(flag.Args(), " "))
	if goal == "" {
		fmt.Fprintln(os.Stderr, "missing goal")
		os.Exit(2)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	if *repoRoot != "" {
		cfg.RepoRoot = *repoRoot
	}
	if *modelProvider != "" {
		cfg.Model.Provider = *modelProvider
	}
	if *modelName != "" {
		cfg.Model.Model = *modelName
	}
	if *baseURL != "" {
		cfg.Model.BaseURL = *baseURL
	}
	if *apiKeyEnv != "" {
		cfg.Model.APIKeyEnv = *apiKeyEnv
	}
	if *maxIterations > 0 {
		cfg.Agent.MaxIterations = *maxIterations
	}
	if *testCmd != "" {
		cfg.Commands.Test = *testCmd
	}

	absRepoRoot, err := cfg.AbsRepoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	r := agent.NewRunner(cfg, absRepoRoot)
	if err := r.Run(ctx, goal); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

