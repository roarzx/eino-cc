package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

type GitDiffTool struct {
	repoRoot string
}

func NewGitDiffTool(repoRoot string) *GitDiffTool {
	return &GitDiffTool{repoRoot: repoRoot}
}

func (t *GitDiffTool) Name() string {
	return "git_diff"
}

func (t *GitDiffTool) Description() string {
	return "Get git diff of changes"
}

type GitDiffArgs struct {
	Path string `json:"path,omitempty"`
}

type GitDiffResult struct {
	Diff string `json:"diff"`
}

func (t *GitDiffTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	parsedArgs, err := ParseArgs[GitDiffArgs](args)
	if err != nil {
		return ErrorResult(err)
	}

	cmdArgs := []string{"diff", "--no-color"}
	if parsedArgs.Path != "" {
		cmdArgs = append(cmdArgs, parsedArgs.Path)
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Dir = t.repoRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return ErrorResult(fmt.Errorf("git diff failed: %s", stderr.String()))
	}

	result := GitDiffResult{
		Diff: stdout.String(),
	}

	return SuccessResult(result)
}