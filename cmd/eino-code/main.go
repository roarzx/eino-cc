package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/eino-ai/eino-claude-code-mvp/internal/agent"
	"github.com/eino-ai/eino-claude-code-mvp/internal/config"
	"github.com/eino-ai/eino-claude-code-mvp/internal/llm"
	"github.com/eino-ai/eino-claude-code-mvp/internal/observe"
	"github.com/eino-ai/eino-claude-code-mvp/internal/tools"
)

func main() {
	// Parse flags first
	flagSet := flag.NewFlagSet("eino-code", flag.ContinueOnError)
	repoRoot := flagSet.String("repo-root", ".", "Repository root directory")
	dryRun := flagSet.Bool("dry-run", false, "Dry run mode (don't apply patches)")
	mockLLM := flagSet.Bool("mock-llm", false, "Use mock LLM client for testing")
	
	// Parse flags, leaving non-flag arguments as positional args
	if err := flagSet.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}
	
	// Get goal from positional arguments
	if flagSet.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s <goal> [--repo-root <path>] [--dry-run] [--mock-llm]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s \"fix login bug\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s --repo-root ./test_project --mock-llm \"fix undefined function\"\n", os.Args[0])
		os.Exit(1)
	}
	
	goal := flagSet.Arg(0)

	// Determine config path: if repoRoot is a directory, look for config.yaml in it
	configPath := *repoRoot
	if fi, err := os.Stat(*repoRoot); err == nil && fi.IsDir() {
		configPath = filepath.Join(*repoRoot, "config.yaml")
		if _, err := os.Stat(configPath); err != nil {
			// Fall back to default config
			configPath = "configs/default.yaml"
		}
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Override repo root if specified and it's a directory
	if *repoRoot != "." {
		if fi, err := os.Stat(*repoRoot); err == nil && fi.IsDir() {
			cfg.RepoRoot = *repoRoot
		}
	}

	// Set dry run mode
	cfg.Agent.DryRun = *dryRun

	fmt.Printf("Goal: %s\n", goal)
	fmt.Printf("Repo root: %s\n", cfg.RepoRoot)
	fmt.Printf("Max iterations: %d\n", cfg.Agent.MaxIterations)

	ctx := context.Background()
	if err := run(ctx, cfg, goal, *mockLLM); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config.Config, goal string, mockLLM bool) error {
	fmt.Println("Agent starting...")

	// Setup logging
	logger := observe.NewLogger("eino-code")

	// Create tool registry
	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(tools.NewRepoTreeTool(cfg.RepoRoot))
	toolRegistry.Register(tools.NewSearchCodeTool(cfg.RepoRoot))
	toolRegistry.Register(tools.NewOpenFileTool(cfg.RepoRoot))
	toolRegistry.Register(tools.NewApplyPatchTool(cfg.RepoRoot))
	toolRegistry.Register(tools.NewGitDiffTool(cfg.RepoRoot))
	toolRegistry.Register(tools.NewRunCmdTool(cfg.RepoRoot))

	// Create agent state
	state := agent.NewRunState(cfg.RepoRoot, goal, cfg.Agent.MaxIterations)

	// Create LLM client
	var llmClient llm.Client
	if mockLLM {
		llmClient = llm.NewMockClient()
		logger.Info("Using mock LLM client for testing")
	} else if cfg.Model.APIKey != "" && cfg.Model.APIKey != "your-api-key-here" {
		llmClient = llm.NewOpenAIClient(cfg.Model.APIKey, cfg.Model.Model)
		logger.Info("LLM client initialized with model: %s", cfg.Model.Model)
	} else {
		logger.Warn("No valid API key configured, using fallback mode")
	}

	// Create and run agent
	runner := agent.NewRunner(state, toolRegistry, logger, llmClient)
	if err := runner.Run(ctx); err != nil {
		return fmt.Errorf("agent run failed: %w", err)
	}

	// Output summary
	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Goal: %s\n", state.UserGoal)
	fmt.Printf("Status: %s\n", state.DoneReason)
	fmt.Printf("Iterations: %d/%d\n", state.Iteration, state.MaxIterations)
	fmt.Printf("Tests passed: %v\n", state.TestsPassed)
	fmt.Printf("Modified files: %d\n", len(state.ModifiedFiles))

	return nil
}