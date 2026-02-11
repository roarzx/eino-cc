package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type OpenFileParams struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type OpenFileData struct {
	Path      string `json:"path"`
	Offset    int    `json:"offset"`
	Limit     int    `json:"limit"`
	Content   string `json:"content"`
	LineCount int    `json:"line_count"`
}

func OpenFile(_ context.Context, repoRoot string, params *OpenFileParams) Result {
	if params == nil || strings.TrimSpace(params.Path) == "" {
		return Err("missing path")
	}
	absPath, err := safeJoin(repoRoot, params.Path)
	if err != nil {
		return Err(err.Error())
	}

	f, err := os.Open(absPath)
	if err != nil {
		return Err(fmt.Sprintf("open file: %v", err))
	}
	defer f.Close()

	offset := params.Offset
	if offset <= 0 {
		offset = 1
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}

	scanner := bufio.NewScanner(f)
	lineNo := 0
	wrote := 0
	var b strings.Builder
	truncated := false

	for scanner.Scan() {
		lineNo++
		if lineNo < offset {
			continue
		}
		if wrote >= limit {
			truncated = true
			break
		}
		wrote++
		fmt.Fprintf(&b, "%d→%s\n", lineNo, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return Err(fmt.Sprintf("read file: %v", err))
	}

	data := OpenFileData{
		Path:      filepath.ToSlash(filepath.Clean(params.Path)),
		Offset:    offset,
		Limit:     limit,
		Content:   b.String(),
		LineCount: lineNo,
	}
	return Result{OK: true, Data: data, Meta: &ResultMeta{Truncated: truncated}}
}

func safeJoin(root string, rel string) (string, error) {
	rel = filepath.Clean(rel)
	if rel == "." || rel == string(filepath.Separator) {
		return "", fmt.Errorf("invalid path")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute path not allowed")
	}
	abs := filepath.Join(root, rel)
	abs, err := filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rootAbs = filepath.Clean(rootAbs) + string(filepath.Separator)
	if !strings.HasPrefix(abs+string(filepath.Separator), rootAbs) && abs != strings.TrimSuffix(rootAbs, string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes repo_root")
	}
	return abs, nil
}

