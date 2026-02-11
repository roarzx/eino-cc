package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/eino-ai/eino-claude-code-mvp/internal/llm"
	"github.com/eino-ai/eino-claude-code-mvp/internal/observe"
	"github.com/eino-ai/eino-claude-code-mvp/internal/tools"
)

type ReActAgent struct {
	state  *RunState
	tools  *tools.Registry
	logger *observe.Logger
	llm    llm.Client
}

func NewReActAgent(state *RunState, toolRegistry *tools.Registry, logger *observe.Logger, llmClient llm.Client) *ReActAgent {
	return &ReActAgent{
		state:  state,
		tools:  toolRegistry,
		logger: logger,
		llm:    llmClient,
	}
}

func (a *ReActAgent) ExecuteStep(ctx context.Context) (bool, error) {
	// If we have a patch, apply it
	if a.state.LastPatch != "" {
		a.logger.Info("Applying patch from previous step")
		tool, ok := a.tools.Get("apply_patch")
		if !ok {
			return false, fmt.Errorf("apply_patch tool not found")
		}

		result, err := tool.Execute(ctx, json.RawMessage(fmt.Sprintf(`{"patch": %q}`, a.state.LastPatch)))
		if err != nil {
			return false, err
		}

		var toolResult tools.ToolResult
		if err := json.Unmarshal(result, &toolResult); err != nil {
			return false, err
		}

		if !toolResult.OK {
			a.logger.Error("Failed to apply patch: %s", toolResult.Error)
			// Continue anyway, maybe the patch was invalid
		}

		a.state.LastPatch = ""
	}

	// Run tests
	a.logger.Info("Running tests")
	tool, ok := a.tools.Get("run_cmd")
	if !ok {
		return false, fmt.Errorf("run_cmd tool not found")
	}

	result, err := tool.Execute(ctx, json.RawMessage(`{"command": "go test ./..."}`))
	if err != nil {
		return false, err
	}

	var toolResult tools.ToolResult
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return false, err
	}

	if !toolResult.OK {
		return false, fmt.Errorf("run_cmd failed: %s", toolResult.Error)
	}

	var cmdResult struct {
		ExitCode int    `json:"exit_code"`
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
	}
	if err := json.Unmarshal(toolResult.Data, &cmdResult); err != nil {
		return false, err
	}

	a.state.LastCmd = "go test ./..."
	a.state.LastCmdOut = cmdResult.Stdout
	a.state.LastCmdErr = cmdResult.Stderr

	if cmdResult.ExitCode == 0 {
		a.state.TestsPassed = true
		a.logger.Info("Tests passed")
		return true, nil
	}

	// Tests failed, use LLM to decide next action
	a.logger.Info("Tests failed, consulting LLM for next action")
	
	// Prepare observation for LLM
	observation := fmt.Sprintf("Test failed with exit code %d\nStdout: %s\nStderr: %s", 
		cmdResult.ExitCode, cmdResult.Stdout, cmdResult.Stderr)
	
	// Prepare agent state for LLM
	agentState := &llm.AgentState{
		Iteration:     a.state.Iteration,
		PreviousSteps: a.getPreviousSteps(),
		FilesOpened:   a.getOpenedFiles(),
		TestResults:   observation,
		Goal:          a.state.UserGoal,
	}

	// Use LLM to generate thought if available
	if a.llm != nil {
		a.logger.Info("LLM is available, observation: %s", observation)
		thought, err := a.llm.GenerateThought(ctx, observation, agentState)
		if err != nil {
			a.logger.Error("Failed to generate thought with LLM: %v", err)
			// Fall back to simple analysis
			return a.executeSimpleAnalysis(ctx, cmdResult.Stdout, cmdResult.Stderr)
		}

		a.logger.Info("LLM thought: %s", thought)
		return a.executeLLMThought(ctx, thought)
	} else {
		a.logger.Info("LLM is nil, falling back to simple analysis")
	}

	// Fall back to simple analysis if no LLM
	return a.executeSimpleAnalysis(ctx, cmdResult.Stdout, cmdResult.Stderr)
}

func (a *ReActAgent) analyzeTestFailure(stdout, stderr string) string {
	combined := stdout + "\n" + stderr

	// Simple analysis based on common patterns
	if strings.Contains(combined, "undefined") {
		return "undefined symbol - check imports and function names"
	}
	if strings.Contains(combined, "syntax error") {
		return "syntax error - check code formatting and brackets"
	}
	if strings.Contains(combined, "panic") {
		return "runtime panic - check nil pointers and array bounds"
	}
	if strings.Contains(combined, "timeout") {
		return "test timeout - check for infinite loops or slow operations"
	}
	if strings.Contains(combined, "assertion failed") {
		return "assertion failed - check test expectations match actual behavior"
	}

	return "unknown error - need to examine test output"
}

func (a *ReActAgent) generateNextAction(diagnosis string) string {
	// Simple heuristic for next action
	if strings.Contains(diagnosis, "undefined") {
		return "search_code for missing symbol"
	}
	if strings.Contains(diagnosis, "syntax error") {
		return "open_file to examine syntax"
	}
	if strings.Contains(diagnosis, "panic") {
		return "search_code for panic locations"
	}

	return "search_code for error patterns"
}

func (a *ReActAgent) GeneratePatch(ctx context.Context, filePath, goal string) (string, error) {
	// Check if we have LLM client
	if a.llm == nil {
		// Fallback to simple patch
		return fmt.Sprintf(`diff --git a/%s b/%s
index 0000000..1111111
--- a/%s
+++ b/%s
@@ -1,1 +1,1 @@
-// TODO: implement changes for: %s
+// Implemented: %s
`, filePath, filePath, filePath, filePath, goal, goal), nil
	}
	
	// Get file content if it's opened
	fileContent, ok := a.state.OpenedFiles[filePath]
	if !ok {
		// Try to read the file
		tool, ok := a.tools.Get("open_file")
		if !ok {
			return "", fmt.Errorf("open_file tool not found")
		}
		
		result, err := tool.Execute(ctx, json.RawMessage(fmt.Sprintf(`{"path": %q}`, filePath)))
		if err != nil {
			return "", fmt.Errorf("failed to open file: %w", err)
		}
		
		var toolResult tools.ToolResult
		if err := json.Unmarshal(result, &toolResult); err != nil {
			return "", fmt.Errorf("failed to parse tool result: %w", err)
		}
		
		if !toolResult.OK {
			return "", fmt.Errorf("open_file failed: %s", toolResult.Error)
		}
		
		var fileData struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(toolResult.Data, &fileData); err != nil {
			return "", fmt.Errorf("failed to parse file data: %w", err)
		}
		
		fileContent = fileData.Content
		a.state.AddOpenedFile(filePath, fileContent)
	}
	
	// Prepare agent state for LLM
	agentState := &llm.AgentState{
		Iteration:     a.state.Iteration,
		PreviousSteps: a.getPreviousSteps(),
		FilesOpened:   a.getOpenedFiles(),
		TestResults:   fmt.Sprintf("Stdout: %s\nStderr: %s", a.state.LastCmdOut, a.state.LastCmdErr),
		Goal:          goal,
	}
	
	// Use LLM to generate patch
	patch, err := a.llm.GeneratePatch(ctx, fileContent, a.state.LastCmdErr, agentState)
	if err != nil {
		a.logger.Error("Failed to generate patch with LLM: %v", err)
		return "", err
	}
	
	a.logger.Info("Generated patch with LLM")
	return patch, nil
}

// getPreviousSteps returns a summary of previous attempts
func (a *ReActAgent) getPreviousSteps() []string {
	var steps []string
	for _, attempt := range a.state.Attempts {
		step := fmt.Sprintf("Iteration %d: %s", attempt.Iteration, attempt.Diagnosis)
		steps = append(steps, step)
	}
	return steps
}

// getOpenedFiles returns a list of opened files with their paths
func (a *ReActAgent) getOpenedFiles() []string {
	var files []string
	for path := range a.state.OpenedFiles {
		files = append(files, path)
	}
	return files
}

// executeSimpleAnalysis performs basic analysis without LLM
func (a *ReActAgent) executeSimpleAnalysis(ctx context.Context, stdout, stderr string) (bool, error) {
	diagnosis := a.analyzeTestFailure(stdout, stderr)
	a.logger.Info("Simple analysis diagnosis: %s", diagnosis)
	
	// Add attempt to state
	a.state.AddAttempt("test", stdout, stderr, diagnosis)
	
	// Execute the diagnosis (e.g., search for error patterns)
	if strings.Contains(diagnosis, "search_code") {
		tool, ok := a.tools.Get("search_code")
		if !ok {
			return false, fmt.Errorf("search_code tool not found")
		}
		
		// Search for error patterns
		result, err := tool.Execute(ctx, json.RawMessage(`{"query": "error"}`))
		if err != nil {
			return false, err
		}
		
		a.logger.Info("Search result: %s", result)
		return false, nil
	}
	
	return false, nil
}

// executeLLMThought executes the action suggested by LLM thought
func (a *ReActAgent) executeLLMThought(ctx context.Context, thought string) (bool, error) {
	a.logger.Info("Executing LLM thought: %s", thought)
	
	// Parse the thought to determine action
	a.state.AddAttempt("llm_thought", "", "", thought)
	
	// Simple parsing of thought to determine action
	lowerThought := strings.ToLower(thought)
	
	// Check for search actions
	if strings.Contains(lowerThought, "search") || strings.Contains(lowerThought, "look for") {
		// Extract search query from thought
		query := extractSearchQuery(thought)
		a.logger.Info("Parsed search query: '%s' from thought: '%s'", query, thought)
		if query == "" {
			query = "error" // Default search query
			a.logger.Info("Using default search query: '%s'", query)
		}
		
		tool, ok := a.tools.Get("search_code")
		if !ok {
			return false, fmt.Errorf("search_code tool not found")
		}
		
		a.logger.Info("Executing search for: '%s'", query)
		result, err := tool.Execute(ctx, json.RawMessage(fmt.Sprintf(`{"query": %q}`, query)))
		if err != nil {
			a.logger.Error("Search failed: %v", err)
			return false, err
		}
		
		a.logger.Info("Search result: %s", result)
		return false, nil
	}
	
	// Check for edit/patch actions
	if strings.Contains(lowerThought, "edit") || strings.Contains(lowerThought, "fix") || 
	   strings.Contains(lowerThought, "patch") || strings.Contains(lowerThought, "modify") {
		
		// Extract file path from thought
		filePath := extractFilePath(thought)
		if filePath == "" {
			// If no file specified, check opened files
			files := a.getOpenedFiles()
			if len(files) > 0 {
				filePath = files[0] // Use first opened file
			} else {
				// Search for relevant files
				tool, ok := a.tools.Get("search_code")
				if ok {
					result, err := tool.Execute(ctx, json.RawMessage(`{"query": "error"}`))
					if err == nil {
						var toolResult tools.ToolResult
						if err := json.Unmarshal(result, &toolResult); err == nil && toolResult.OK {
							var searchResult struct {
								Files []struct {
									Path string `json:"path"`
								} `json:"files"`
							}
							if err := json.Unmarshal(toolResult.Data, &searchResult); err == nil && len(searchResult.Files) > 0 {
								filePath = searchResult.Files[0].Path
							}
						}
					}
				}
			}
		}
		
		if filePath == "" {
			a.logger.Warn("Could not determine file to edit from thought: %s", thought)
			return false, nil
		}
		
		// Generate patch using LLM
		patch, err := a.GeneratePatch(ctx, filePath, a.state.UserGoal)
		if err != nil {
			a.logger.Error("Failed to generate patch: %v", err)
			return false, err
		}
		
		// Store patch for next iteration
		a.state.LastPatch = patch
		a.logger.Info("Generated patch for %s", filePath)
		return false, nil
	}
	
	// Check for open file actions
	if strings.Contains(lowerThought, "open") || strings.Contains(lowerThought, "examine") {
		filePath := extractFilePath(thought)
		if filePath == "" {
			a.logger.Warn("Could not determine file to open from thought: %s", thought)
			return false, nil
		}
		
		tool, ok := a.tools.Get("open_file")
		if !ok {
			return false, fmt.Errorf("open_file tool not found")
		}
		
		result, err := tool.Execute(ctx, json.RawMessage(fmt.Sprintf(`{"path": %q}`, filePath)))
		if err != nil {
			return false, err
		}
		
		a.logger.Info("Opened file: %s", result)
		return false, nil
	}
	
	// Default: just log the thought
	a.logger.Info("No specific action determined from thought")
	return false, nil
}

// extractSearchQuery extracts search query from thought
func extractSearchQuery(thought string) string {
	// Simple extraction - look for quoted text after "search" or "look for"
	lower := strings.ToLower(thought)
	
	// Check for "search for X" pattern
	if idx := strings.Index(lower, "search for "); idx != -1 {
		start := idx + len("search for ")
		// Find end of query (next punctuation or end of string)
		end := len(thought)
		for i, ch := range thought[start:] {
			if ch == '.' || ch == ',' || ch == ';' || ch == '!' || ch == '?' || ch == ' ' {
				// Stop at first space after the search term
				if i > 0 {
					end = start + i
					break
				}
			}
		}
		return strings.TrimSpace(thought[start:end])
	}
	
	// Check for "look for X" pattern
	if idx := strings.Index(lower, "look for "); idx != -1 {
		start := idx + len("look for ")
		end := len(thought)
		for i, ch := range thought[start:] {
			if ch == '.' || ch == ',' || ch == ';' || ch == '!' || ch == '?' || ch == ' ' {
				if i > 0 {
					end = start + i
					break
				}
			}
		}
		return strings.TrimSpace(thought[start:end])
	}
	
	// Check for "search X" pattern (without "for")
	if idx := strings.Index(lower, "search "); idx != -1 {
		start := idx + len("search ")
		end := len(thought)
		for i, ch := range thought[start:] {
			if ch == '.' || ch == ',' || ch == ';' || ch == '!' || ch == '?' || ch == ' ' {
				if i > 0 {
					end = start + i
					break
				}
			}
		}
		return strings.TrimSpace(thought[start:end])
	}
	
	return ""
}

// extractFilePath extracts file path from thought
func extractFilePath(thought string) string {
	// Look for file extensions or path patterns
	words := strings.Fields(thought)
	for _, word := range words {
		// Check for common file extensions
		if strings.HasSuffix(word, ".go") || strings.HasSuffix(word, ".js") || 
		   strings.HasSuffix(word, ".ts") || strings.HasSuffix(word, ".py") ||
		   strings.HasSuffix(word, ".java") || strings.HasSuffix(word, ".cpp") ||
		   strings.HasSuffix(word, ".h") || strings.HasSuffix(word, ".rs") {
			// Clean up any punctuation
			clean := strings.Trim(word, ".,;:!?\"'")
			return clean
		}
		
		// Check for path patterns
		if strings.Contains(word, "/") || strings.Contains(word, "\\") {
			clean := strings.Trim(word, ".,;:!?\"'")
			return clean
		}
	}
	
	return ""
}