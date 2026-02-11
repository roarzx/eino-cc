package tools

import (
	"context"
	"strings"
)

type GitDiffData struct {
	Diff string `json:"diff"`
}

func GitDiff(ctx context.Context, repoRoot string) Result {
	out, err := RunCommand(ctx, repoRoot, "git", "diff")
	if err != nil {
		return Err("git diff failed")
	}
	if out.ExitCode != 0 {
		msg := strings.TrimSpace(out.Stderr)
		if msg == "" {
			msg = "git diff failed"
		}
		return Err(msg)
	}
	return OK(GitDiffData{Diff: out.Stdout})
}

