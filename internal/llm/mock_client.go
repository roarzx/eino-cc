package llm

import (
	"context"
	"fmt"
	"strings"
)

// MockClient is a mock implementation of Client for testing
type MockClient struct{}

func NewMockClient() *MockClient {
	return &MockClient{}
}

func (m *MockClient) GeneratePlan(ctx context.Context, goal string, state *AgentState) (string, error) {
	// Generate different plans based on the goal
	if contains(goal, "undefined") || contains(goal, "function") {
		return `{
  "steps": [
    {
      "tool": "search_code",
      "args": {
        "query": "undefinedFunction"
      }
    },
    {
      "tool": "open_file",
      "args": {
        "path": "main.go"
      }
    },
    {
      "tool": "run_cmd",
      "args": {
        "command": "go run main.go"
      }
    }
  ],
  "success_criteria": ["program compiles and runs without undefined function error"]
}`, nil
	}
	
	// Default plan for other goals
	return `{
  "steps": [
    {
      "tool": "repo_tree",
      "args": {}
    },
    {
      "tool": "run_cmd",
      "args": {
        "command": "go test ./..."
      }
    }
  ],
  "success_criteria": ["tests pass"]
}`, nil
}

func (m *MockClient) GenerateThought(ctx context.Context, observation string, state *AgentState) (string, error) {
	// Return different thoughts based on observation content
	switch {
	case contains(observation, "undefined"):
		return "search for undefinedFunction to find where it's called", nil
	case contains(observation, "search_code") && contains(observation, "results"):
		return "open main.go to examine the code", nil
	case contains(observation, "open_file") && contains(observation, "main.go"):
		return "edit main.go to add the undefinedFunction definition", nil
	case contains(observation, "run_cmd") && contains(observation, "exit code: 2"):
		return "The go run command failed. I need to fix the undefined function error in main.go", nil
	default:
		return "search for errors in the codebase", nil
	}
}

func (m *MockClient) GeneratePatch(ctx context.Context, fileContent string, errorMessage string, state *AgentState) (string, error) {
	// Generate a patch to add the missing function
	if contains(errorMessage, "undefinedFunction") || contains(fileContent, "undefinedFunction()") {
		// Check if fmt import is needed
		patch := `--- a/main.go
+++ b/main.go
@@ -1,7 +1,12 @@
 package main

+import "fmt"
+
 func main() {
-    // This function is undefined
-    undefinedFunction()
+	// This function is undefined
+	undefinedFunction()
+}
+
+func undefinedFunction() {
+	fmt.Println("undefinedFunction is now defined!")
 }`
		return patch, nil
	}
	
	return "", fmt.Errorf("no patch generated for error: %s", errorMessage)
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}