package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type SearchCodeTool struct {
	repoRoot string
}

func NewSearchCodeTool(repoRoot string) *SearchCodeTool {
	return &SearchCodeTool{repoRoot: repoRoot}
}

func (t *SearchCodeTool) Name() string {
	return "search_code"
}

func (t *SearchCodeTool) Description() string {
	return "Search for code using ripgrep"
}

type SearchCodeArgs struct {
	Query   string `json:"query"`
	Pattern string `json:"pattern,omitempty"`
}

type SearchResult struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

type SearchCodeResult struct {
	Results []SearchResult `json:"results"`
}

func (t *SearchCodeTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	parsedArgs, err := ParseArgs[SearchCodeArgs](args)
	if err != nil {
		return ErrorResult(err)
	}

	if parsedArgs.Query == "" {
		return ErrorResult(fmt.Errorf("query is required"))
	}

	// Try to use ripgrep if available
	if _, err := exec.LookPath("rg"); err == nil {
		return t.searchWithRipgrep(ctx, parsedArgs)
	}

	// Fallback to simple file search
	return t.searchWithGo(ctx, parsedArgs)
}

func (t *SearchCodeTool) searchWithRipgrep(ctx context.Context, args SearchCodeArgs) (json.RawMessage, error) {
	cmdArgs := []string{
		"--color", "never",
		"--line-number",
		"--no-heading",
		args.Query,
		t.repoRoot,
	}

	if args.Pattern != "" {
		cmdArgs = append([]string{"-g", args.Pattern}, cmdArgs...)
	}

	cmd := exec.CommandContext(ctx, "rg", cmdArgs...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// ripgrep returns 1 when no matches found
			return SuccessResult(SearchCodeResult{Results: []SearchResult{}})
		}
		return ErrorResult(fmt.Errorf("ripgrep failed: %w", err))
	}

	results := []SearchResult{}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 3)
		if len(parts) >= 3 {
			file := strings.TrimPrefix(parts[0], t.repoRoot+"/")
			lineNum := 0
			fmt.Sscanf(parts[1], "%d", &lineNum)
			content := parts[2]
			results = append(results, SearchResult{
				File:    file,
				Line:    lineNum,
				Content: content,
			})
		}
	}

	return SuccessResult(SearchCodeResult{Results: results})
}

func (t *SearchCodeTool) searchWithGo(ctx context.Context, args SearchCodeArgs) (json.RawMessage, error) {
	results := []SearchResult{}

	err := filepath.Walk(t.repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if args.Pattern != "" {
			matched, err := filepath.Match(args.Pattern, filepath.Base(path))
			if err != nil || !matched {
				return nil
			}
		}

		// Skip binary files and executables
		ext := filepath.Ext(path)
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".pdf" {
			return nil
		}
		
		// Skip files without extensions that might be executables
		if ext == "" {
			// Check if file is executable
			if info.Mode()&0111 != 0 {
				return nil
			}
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip unreadable files
		}

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if strings.Contains(line, args.Query) {
				relPath, _ := filepath.Rel(t.repoRoot, path)
				results = append(results, SearchResult{
					File:    relPath,
					Line:    i + 1,
					Content: line,
				})
			}
		}

		return nil
	})

	if err != nil {
		return ErrorResult(fmt.Errorf("walk failed: %w", err))
	}

	return SuccessResult(SearchCodeResult{Results: results})
}