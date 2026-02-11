package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

type Client interface {
	GeneratePlan(ctx context.Context, goal string, state *AgentState) (string, error)
	GenerateThought(ctx context.Context, observation string, state *AgentState) (string, error)
	GeneratePatch(ctx context.Context, fileContent string, errorMessage string, state *AgentState) (string, error)
}

type AgentState struct {
	Iteration     int
	PreviousSteps []string
	FilesOpened   []string
	TestResults   string
	Goal          string
}

type OpenAIClient struct {
	client *openai.Client
	model  string
}

func NewOpenAIClient(apiKey, model string) *OpenAIClient {
	config := openai.DefaultConfig(apiKey)
	// For vision-proxy or other custom endpoints, we'd configure here
	// config.BaseURL = "https://api.vision-proxy.com/v1"
	
	return &OpenAIClient{
		client: openai.NewClientWithConfig(config),
		model:  model,
	}
}

func (c *OpenAIClient) GeneratePlan(ctx context.Context, goal string, state *AgentState) (string, error) {
	prompt := fmt.Sprintf(`You are an AI coding assistant. Given the goal: "%s"

Analyze the codebase and create a step-by-step plan to achieve this goal. The plan should be in JSON format with the following structure:
{
  "steps": [
    {
      "tool": "tool_name",
      "args": {
        "arg1": "value1",
        "arg2": "value2"
      }
    }
  ],
  "success_criteria": ["criteria1", "criteria2"]
}

Available tools: repo_tree, search_code, open_file, apply_patch, git_diff, run_cmd

Return ONLY the JSON plan, no other text.`, goal)

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are an expert software engineer and coding assistant.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: 0.1,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate plan: %w", err)
	}

	return resp.Choices[0].Message.Content, nil
}

func (c *OpenAIClient) GenerateThought(ctx context.Context, observation string, state *AgentState) (string, error) {
	prompt := fmt.Sprintf(`You are an AI coding assistant working on: "%s"

Current state:
- Iteration: %d
- Previous steps: %v
- Files opened: %v
- Test results: %s

Latest observation: %s

Based on this observation, decide what to do next. You can:
1. Use a tool to gather more information
2. Generate a patch to fix an issue
3. Run tests to verify changes

Available tools: repo_tree, search_code, open_file, apply_patch, git_diff, run_cmd

Return your thought in this format:
THOUGHT: [Your analysis of the situation and what to do next]
ACTION: [Tool name or "generate_patch" or "done"]
ARGS: [JSON arguments for the tool, or empty string]

If you think the goal is achieved, use ACTION: "done".`, 
		state.Goal, state.Iteration, state.PreviousSteps, state.FilesOpened, 
		state.TestResults, observation)

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are an expert software engineer following the ReAct pattern.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: 0.1,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate thought: %w", err)
	}

	return resp.Choices[0].Message.Content, nil
}

func (c *OpenAIClient) GeneratePatch(ctx context.Context, fileContent string, errorMessage string, state *AgentState) (string, error) {
	prompt := fmt.Sprintf(`You are an AI coding assistant fixing an error.

File content:
%s

Error message:
%s

Goal: %s

Generate a patch in unified diff format to fix the error. The patch should:
1. Fix the specific error shown
2. Maintain code style and conventions
3. Not break existing functionality

Return ONLY the patch in unified diff format, no other text. Example format:
--- a/file.go
+++ b/file.go
@@ -1,5 +1,5 @@
 package main

-func oldFunction() {
+func newFunction() {
     // implementation
 }`, fileContent, errorMessage, state.Goal)

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are an expert at generating code patches to fix errors.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: 0.1,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate patch: %w", err)
	}

	// Clean up the response to ensure it's valid patch format
	patch := strings.TrimSpace(resp.Choices[0].Message.Content)
	if !strings.HasPrefix(patch, "--- ") {
		// Try to extract patch from response
		lines := strings.Split(patch, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "--- ") {
				patch = strings.Join(lines[i:], "\n")
				break
			}
		}
	}

	return patch, nil
}