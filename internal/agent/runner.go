package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/eino-ai/eino-claude-code-mvp/internal/llm"
	"github.com/eino-ai/eino-claude-code-mvp/internal/observe"
	"github.com/eino-ai/eino-claude-code-mvp/internal/tools"
)

type Runner struct {
	state    *RunState
	tools    *tools.Registry
	logger   *observe.Logger
	llm      llm.Client
}

func NewRunner(state *RunState, toolRegistry *tools.Registry, logger *observe.Logger, llmClient llm.Client) *Runner {
	return &Runner{
		state:  state,
		tools:  toolRegistry,
		logger: logger,
		llm:    llmClient,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	r.logger.Info("Starting agent run: %s", r.state.UserGoal)

	// Step 1: Generate plan
	plan, err := r.generatePlan(ctx)
	if err != nil {
		return fmt.Errorf("failed to generate plan: %w", err)
	}
	r.state.PlanJSON = plan

	// Step 2: Execute iterations
	for r.state.NextIteration() && !r.state.IsDone() {
		r.logger.Info("Starting iteration %d", r.state.Iteration)

		if err := r.executeIteration(ctx); err != nil {
			return fmt.Errorf("iteration %d failed: %w", r.state.Iteration, err)
		}

		if r.state.TestsPassed {
			r.state.DoneReason = "tests passed"
			break
		}
	}

	if !r.state.TestsPassed {
		r.state.DoneReason = "max iterations reached"
	}

	// Step 3: Output final diff
	if err := r.outputFinalDiff(ctx); err != nil {
		r.logger.Error("Failed to get final diff: %v", err)
	}

	r.logger.Info("Agent run completed: %s", r.state.DoneReason)
	return nil
}

func (r *Runner) generatePlan(ctx context.Context) (string, error) {
	if r.llm == nil {
		// Fallback to simple plan if LLM not configured
		plan := map[string]interface{}{
			"steps": []map[string]interface{}{
				{
					"tool": "search_code",
					"args": map[string]interface{}{
						"query": "TODO",
					},
				},
				{
					"tool": "run_cmd",
					"args": map[string]interface{}{
						"command": "go test ./...",
					},
				},
			},
			"success_criteria": []string{"tests pass"},
		}

		jsonPlan, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return "", err
		}

		r.logger.Info("Generated fallback plan:\n%s", string(jsonPlan))
		return string(jsonPlan), nil
	}

	// Use LLM to generate plan
	agentState := &llm.AgentState{
		Iteration:     0,
		PreviousSteps: []string{},
		FilesOpened:   []string{},
		TestResults:   "",
		Goal:          r.state.UserGoal,
	}

	plan, err := r.llm.GeneratePlan(ctx, r.state.UserGoal, agentState)
	if err != nil {
		r.logger.Error("Failed to generate plan with LLM: %v", err)
		return "", err
	}

	r.logger.Info("Generated plan with LLM:\n%s", plan)
	return plan, nil
}

func (r *Runner) executeIteration(ctx context.Context) error {
	// Create ReAct agent for this iteration
	agent := NewReActAgent(r.state, r.tools, r.logger, r.llm)
	
	// Execute ReAct step
	done, err := agent.ExecuteStep(ctx)
	if err != nil {
		return fmt.Errorf("ReAct step failed: %w", err)
	}

	if done {
		r.state.TestsPassed = true
		r.state.DoneReason = "goal achieved"
	}

	return nil
}

func (r *Runner) outputFinalDiff(ctx context.Context) error {
	tool, ok := r.tools.Get("git_diff")
	if !ok {
		return fmt.Errorf("git_diff tool not found")
	}

	result, err := tool.Execute(ctx, json.RawMessage(`{}`))
	if err != nil {
		return err
	}

	var toolResult tools.ToolResult
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return err
	}

	if !toolResult.OK {
		return fmt.Errorf("git_diff failed: %s", toolResult.Error)
	}

	var diffResult struct {
		Diff string `json:"diff"`
	}
	if err := json.Unmarshal(toolResult.Data, &diffResult); err != nil {
		return err
	}

	fmt.Println("\n=== Final Diff ===")
	fmt.Println(diffResult.Diff)
	fmt.Println("==================")

	return nil
}