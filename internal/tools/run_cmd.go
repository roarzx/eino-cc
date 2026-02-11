package tools

import (
	"context"
	"strings"
)

type RunCmdParams struct {
	Name string `json:"name"`
}

type RunCmdData struct {
	Name     string `json:"name"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

func RunCmd(ctx context.Context, repoRoot string, allowed map[string]string, params *RunCmdParams) Result {
	if params == nil || strings.TrimSpace(params.Name) == "" {
		return Err("missing name")
	}
	cmd, ok := allowed[params.Name]
	if !ok || strings.TrimSpace(cmd) == "" {
		return Err("unknown command")
	}

	out, err := RunCommand(ctx, repoRoot, "bash", "-lc", cmd)
	if err != nil {
		return Err("command execution failed")
	}
	data := RunCmdData{Name: params.Name, ExitCode: out.ExitCode, Stdout: out.Stdout, Stderr: out.Stderr}
	if out.ExitCode != 0 {
		return Result{OK: false, Error: "command failed", Data: data}
	}
	return OK(data)
}

