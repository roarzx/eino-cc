package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type ApplyPatchParams struct {
	Patch string `json:"patch"`
}

type ApplyPatchData struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

func ApplyPatch(ctx context.Context, repoRoot string, params *ApplyPatchParams) Result {
	if params == nil || strings.TrimSpace(params.Patch) == "" {
		return Err("missing patch")
	}

	cmd := exec.CommandContext(ctx, "git", "apply", "--whitespace=nowarn")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(params.Patch)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			return Err(fmt.Sprintf("git apply failed: %v", err))
		}
	}

	data := ApplyPatchData{ExitCode: exitCode, Stdout: stdout.String(), Stderr: stderr.String()}
	if exitCode != 0 {
		return Result{OK: false, Error: "git apply failed", Data: data}
	}
	return OK(data)
}

