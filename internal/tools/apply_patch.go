package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

type ApplyPatchTool struct {
	repoRoot string
}

func NewApplyPatchTool(repoRoot string) *ApplyPatchTool {
	return &ApplyPatchTool{repoRoot: repoRoot}
}

func (t *ApplyPatchTool) Name() string {
	return "apply_patch"
}

func (t *ApplyPatchTool) Description() string {
	return "Apply a unified diff patch using git apply"
}

type ApplyPatchArgs struct {
	Patch string `json:"patch"`
}

type ApplyPatchResult struct {
	Applied bool   `json:"applied"`
	Message string `json:"message"`
}

func (t *ApplyPatchTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	parsedArgs, err := ParseArgs[ApplyPatchArgs](args)
	if err != nil {
		return ErrorResult(err)
	}

	if parsedArgs.Patch == "" {
		return ErrorResult(fmt.Errorf("patch is required"))
	}

	// Create temporary patch file
	tmpFile, err := os.CreateTemp("", "patch-*.diff")
	if err != nil {
		return ErrorResult(fmt.Errorf("failed to create temp file: %w", err))
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(parsedArgs.Patch)); err != nil {
		return ErrorResult(fmt.Errorf("failed to write patch: %w", err))
	}
	tmpFile.Close()

	// Run git apply
	cmd := exec.CommandContext(ctx, "git", "apply", "--check", tmpFile.Name())
	cmd.Dir = t.repoRoot
	var checkErr bytes.Buffer
	cmd.Stderr = &checkErr

	if err := cmd.Run(); err != nil {
		return ErrorResult(fmt.Errorf("patch check failed: %s", checkErr.String()))
	}

	// Actually apply the patch
	cmd = exec.CommandContext(ctx, "git", "apply", tmpFile.Name())
	cmd.Dir = t.repoRoot
	var applyErr bytes.Buffer
	cmd.Stderr = &applyErr

	if err := cmd.Run(); err != nil {
		return ErrorResult(fmt.Errorf("patch application failed: %s", applyErr.String()))
	}

	result := ApplyPatchResult{
		Applied: true,
		Message: "Patch applied successfully",
	}

	return SuccessResult(result)
}