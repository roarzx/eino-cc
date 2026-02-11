package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type RepoTreeTool struct {
	repoRoot string
}

func NewRepoTreeTool(repoRoot string) *RepoTreeTool {
	return &RepoTreeTool{repoRoot: repoRoot}
}

func (t *RepoTreeTool) Name() string {
	return "repo_tree"
}

func (t *RepoTreeTool) Description() string {
	return "Get directory structure of the repository"
}

type RepoTreeArgs struct {
	Path string `json:"path,omitempty"`
}

type RepoTreeResult struct {
	Path    string   `json:"path"`
	Files   []string `json:"files"`
	Subdirs []string `json:"subdirs"`
}

func (t *RepoTreeTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	parsedArgs, err := ParseArgs[RepoTreeArgs](args)
	if err != nil {
		return ErrorResult(err)
	}

	path := t.repoRoot
	if parsedArgs.Path != "" {
		path = filepath.Join(t.repoRoot, parsedArgs.Path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return ErrorResult(fmt.Errorf("failed to read directory %s: %w", path, err))
	}

	result := RepoTreeResult{
		Path:    path,
		Files:   []string{},
		Subdirs: []string{},
	}

	for _, entry := range entries {
		if entry.IsDir() {
			result.Subdirs = append(result.Subdirs, entry.Name())
		} else {
			result.Files = append(result.Files, entry.Name())
		}
	}

	return SuccessResult(result)
}