package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type OpenFileTool struct {
	repoRoot string
}

func NewOpenFileTool(repoRoot string) *OpenFileTool {
	return &OpenFileTool{repoRoot: repoRoot}
}

func (t *OpenFileTool) Name() string {
	return "open_file"
}

func (t *OpenFileTool) Description() string {
	return "Read file contents"
}

type OpenFileArgs struct {
	Path string `json:"path"`
}

type OpenFileResult struct {
	Path     string `json:"path"`
	Contents string `json:"contents"`
	Size     int64  `json:"size"`
}

func (t *OpenFileTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	parsedArgs, err := ParseArgs[OpenFileArgs](args)
	if err != nil {
		return ErrorResult(err)
	}

	if parsedArgs.Path == "" {
		return ErrorResult(fmt.Errorf("path is required"))
	}

	fullPath := filepath.Join(t.repoRoot, parsedArgs.Path)
	info, err := os.Stat(fullPath)
	if err != nil {
		return ErrorResult(fmt.Errorf("file not found: %s", parsedArgs.Path))
	}

	if info.IsDir() {
		return ErrorResult(fmt.Errorf("path is a directory: %s", parsedArgs.Path))
	}

	// Check file size to avoid reading huge files
	if info.Size() > 10*1024*1024 { // 10MB limit
		return ErrorResult(fmt.Errorf("file too large: %d bytes", info.Size()))
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return ErrorResult(fmt.Errorf("failed to read file: %w", err))
	}

	result := OpenFileResult{
		Path:     parsedArgs.Path,
		Contents: string(content),
		Size:     info.Size(),
	}

	return SuccessResult(result)
}