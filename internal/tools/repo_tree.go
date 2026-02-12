package tools

import (
	"context"
	"path/filepath"
	"strings"
)

type RepoTreeParams struct {
	MaxEntries int `json:"max_entries,omitempty"`
}

type RepoTreeEntry struct {
	Path string `json:"path"`
}

type RepoTreeData struct {
	Entries []RepoTreeEntry `json:"entries"`
}

func RepoTree(ctx context.Context, repoRoot string, params *RepoTreeParams) Result {
	maxEntries := 500
	if params != nil && params.MaxEntries > 0 {
		maxEntries = params.MaxEntries
	}

	out, err := RunCommand(ctx, repoRoot, "git", "ls-files")
	if err == nil && out.ExitCode == 0 {
		lines := splitLines(out.Stdout)
		entries := make([]RepoTreeEntry, 0, minInt(len(lines), maxEntries))
		for _, p := range lines {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			entries = append(entries, RepoTreeEntry{Path: filepath.ToSlash(p)})
			if len(entries) >= maxEntries {
				return Result{OK: true, Data: RepoTreeData{Entries: entries}, Meta: &ResultMeta{Truncated: true}}
			}
		}
		return OK(RepoTreeData{Entries: entries})
	}

	return Err("repo_tree requires a git repository (git ls-files failed)")
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
