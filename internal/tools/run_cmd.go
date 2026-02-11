package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type RunCmdTool struct {
	repoRoot string
}

func NewRunCmdTool(repoRoot string) *RunCmdTool {
	return &RunCmdTool{repoRoot: repoRoot}
}

func (t *RunCmdTool) Name() string {
	return "run_cmd"
}

func (t *RunCmdTool) Description() string {
	return "Execute a shell command"
}

type RunCmdArgs struct {
	Command string `json:"command"`
}

type RunCmdResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

func (t *RunCmdTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	parsedArgs, err := ParseArgs[RunCmdArgs](args)
	if err != nil {
		return ErrorResult(err)
	}

	if parsedArgs.Command == "" {
		return ErrorResult(fmt.Errorf("command is required"))
	}

	// Split command into parts for exec
	parts := strings.Fields(parsedArgs.Command)
	if len(parts) == 0 {
		return ErrorResult(fmt.Errorf("invalid command"))
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = t.repoRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return ErrorResult(fmt.Errorf("command execution failed: %w", err))
		}
	}

	result := RunCmdResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}

	return SuccessResult(result)
}