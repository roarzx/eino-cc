package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"eino-cc/internal/config"
	repoTools "eino-cc/internal/tools"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
)

type Runner struct {
	Cfg      *config.Config
	RepoRoot string
}

func NewRunner(cfg *config.Config, repoRoot string) *Runner {
	return &Runner{Cfg: cfg, RepoRoot: repoRoot}
}

func (r *Runner) Run(ctx context.Context, goal string) error {
	if strings.TrimSpace(goal) == "" {
		return errors.New("goal is empty")
	}
	if r.Cfg == nil {
		return errors.New("config is nil")
	}
	if strings.TrimSpace(r.RepoRoot) == "" {
		return errors.New("repo_root is empty")
	}

	if out, err := repoTools.RunCommand(ctx, r.RepoRoot, "git", "rev-parse", "--is-inside-work-tree"); err != nil || out.ExitCode != 0 {
		return fmt.Errorf("repo_root is not a git repository: %s", r.RepoRoot)
	}

	provider := strings.ToLower(strings.TrimSpace(r.Cfg.Model.Provider))
	if provider == "" {
		provider = "deepseek"
	}

	apiKeyEnv := strings.TrimSpace(r.Cfg.Model.APIKeyEnv)
	if apiKeyEnv == "" {
		if provider == "deepseek" {
			apiKeyEnv = "DEEPSEEK_KEY"
		} else {
			apiKeyEnv = "OPENAI_API_KEY"
		}
	}
	apiKey := os.Getenv(apiKeyEnv)
	if apiKey == "" {
		return fmt.Errorf("missing api key env %s", apiKeyEnv)
	}

	modelName := strings.TrimSpace(r.Cfg.Model.Model)
	if modelName == "" {
		if provider == "deepseek" {
			modelName = "deepseek-chat"
		} else {
			modelName = "gpt-4o"
		}
	}

	baseURL := strings.TrimSpace(r.Cfg.Model.BaseURL)
	if baseURL == "" && provider == "deepseek" {
		baseURL = "https://api.deepseek.com"
	}

	var chatModel model.ToolCallingChatModel
	switch provider {
	case "openai":
		m, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
			APIKey:  apiKey,
			Model:   modelName,
			BaseURL: baseURL,
		})
		if err != nil {
			return fmt.Errorf("create chat model failed: %w", err)
		}
		chatModel = m
	case "deepseek":
		m, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
			APIKey:  apiKey,
			Model:   modelName,
			BaseURL: baseURL,
		})
		if err != nil {
			return fmt.Errorf("create chat model failed: %w", err)
		}
		chatModel = m
	default:
		return fmt.Errorf("unsupported model provider: %s", provider)
	}

	allowed := map[string]string{
		"test": r.Cfg.Commands.Test,
	}
	toolset := newRepoToolset(r.RepoRoot, allowed)
	tools, err := toolset.Tools(ctx)
	if err != nil {
		return fmt.Errorf("build tools failed: %w", err)
	}

	maxIterations := r.Cfg.Agent.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 2
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "eino_code",
		Description:   "Coding agent for applying minimal diffs and running tests",
		Instruction:   SystemPrompt(r.RepoRoot, maxIterations),
		Model:         chatModel,
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
		MaxIterations: maxIterations,
	})
	if err != nil {
		return fmt.Errorf("create agent failed: %w", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
	iter := runner.Query(ctx, goal)
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		msg, _, err := adk.GetMessage(event)
		if err != nil {
			continue
		}
		if strings.TrimSpace(msg.Content) != "" {
			fmt.Println(msg.Content)
		}
	}

	testRes := repoTools.RunCmd(ctx, r.RepoRoot, allowed, &repoTools.RunCmdParams{Name: "test"})
	if !testRes.OK {
		return fmt.Errorf("tests failed: %s", testRes.JSON())
	}

	diffRes := repoTools.GitDiff(ctx, r.RepoRoot)
	fmt.Println(diffRes.JSON())
	return nil
}
