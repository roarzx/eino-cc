package agent

import (
	"context"
	"fmt"
	"strings"

	repoTools "eino-cc/internal/tools"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
)

type repoToolset struct {
	repoRoot          string
	allowed           map[string]string
	applied           bool
	requirePatch      bool
	openedNonTestFile bool
	lastOpenedPath    string
	lastOpenedContent string
	trace             func(string)
}

func newRepoToolset(repoRoot string, allowed map[string]string) *repoToolset {
	return &repoToolset{repoRoot: repoRoot, allowed: allowed}
}

func (t *repoToolset) ResetProgress(keepOpen bool) {
	t.applied = false
	if !keepOpen {
		t.openedNonTestFile = false
		t.lastOpenedPath = ""
		t.lastOpenedContent = ""
	}
}

func (t *repoToolset) RequirePatch(require bool) {
	t.requirePatch = require
}

type toolsetOptions struct {
	includeApplyPatch bool
}

type repoTreeArgs struct {
	MaxEntries int `json:"max_entries,omitempty" jsonschema:"description=maximum file entries to return"`
}

type searchCodeArgs struct {
	Query      string `json:"query" jsonschema:"description=search query, required"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"description=maximum matches to return"`
}

type openFileArgs struct {
	Path   string `json:"path" jsonschema:"description=file path relative to repo_root, required"`
	Offset int    `json:"offset,omitempty" jsonschema:"description=starting line number, 1-based"`
	Limit  int    `json:"limit,omitempty" jsonschema:"description=maximum lines to return"`
}

type applyPatchArgs struct {
	Patch string `json:"patch" jsonschema:"description=unified diff starting with diff --git"`
}

type runCmdArgs struct {
	Name string `json:"name" jsonschema:"description=command name; allowed: test, fmt"`
}

func (t *repoToolset) Tools(ctx context.Context) ([]tool.BaseTool, error) {
	return t.toolsWithOptions(ctx, toolsetOptions{includeApplyPatch: true})
}

func (t *repoToolset) ToolsForProposal(ctx context.Context) ([]tool.BaseTool, error) {
	return t.toolsWithOptions(ctx, toolsetOptions{includeApplyPatch: false})
}

func (t *repoToolset) SetTrace(trace func(string)) {
	t.trace = trace
}

func (t *repoToolset) toolsWithOptions(ctx context.Context, opts toolsetOptions) ([]tool.BaseTool, error) {
	repoTreeTool, err := toolutils.InferTool("repo_tree", "List files in the git repository", func(ctx context.Context, args *repoTreeArgs) (string, error) {
		if t.trace != nil {
			t.trace(fmt.Sprintf("【repo_tree】max_entries=%d", args.MaxEntries))
		}
		res := repoTools.RepoTree(ctx, t.repoRoot, &repoTools.RepoTreeParams{MaxEntries: args.MaxEntries})
		if t.trace != nil {
			t.trace(summarizeResult("repo_tree", res))
		}
		return res.JSON(), nil
	})
	if err != nil {
		return nil, err
	}

	searchTool, err := toolutils.InferTool("search_code", "Search code using ripgrep", func(ctx context.Context, args *searchCodeArgs) (string, error) {
		if t.trace != nil {
			t.trace(fmt.Sprintf("【search_code】query=%q max_results=%d", args.Query, args.MaxResults))
		}
		res := repoTools.SearchCode(ctx, t.repoRoot, &repoTools.SearchCodeParams{Query: args.Query, MaxResults: args.MaxResults})
		if t.trace != nil {
			t.trace(summarizeResult("search_code", res))
		}
		return res.JSON(), nil
	})
	if err != nil {
		return nil, err
	}

	openFileTool, err := toolutils.InferTool("open_file", "Open and read a file", func(ctx context.Context, args *openFileArgs) (string, error) {
		if t.trace != nil {
			t.trace(fmt.Sprintf("【open_file】path=%q offset=%d limit=%d", args.Path, args.Offset, args.Limit))
		}
		res := repoTools.OpenFile(ctx, t.repoRoot, &repoTools.OpenFileParams{Path: args.Path, Offset: args.Offset, Limit: args.Limit})
		if args != nil && args.Path != "" && !strings.HasSuffix(args.Path, "_test.go") {
			t.openedNonTestFile = true
		}
		if data, ok := res.Data.(repoTools.OpenFileData); ok {
			t.lastOpenedPath = data.Path
			t.lastOpenedContent = stripLineNumbers(data.Content)
		} else if dataPtr, okPtr := res.Data.(*repoTools.OpenFileData); okPtr && dataPtr != nil {
			t.lastOpenedPath = dataPtr.Path
			t.lastOpenedContent = stripLineNumbers(dataPtr.Content)
		}
		if t.trace != nil {
			t.trace(summarizeResult("open_file", res))
		}
		return res.JSON(), nil
	})
	if err != nil {
		return nil, err
	}

	var applyPatchTool tool.BaseTool
	if opts.includeApplyPatch {
		toolImpl, err := toolutils.InferTool("apply_patch", "Apply a unified diff patch with git apply", func(ctx context.Context, args *applyPatchArgs) (string, error) {
			if t.trace != nil {
				t.trace(fmt.Sprintf("【apply_patch】bytes=%d", len(args.Patch)))
			}
			res := repoTools.ApplyPatch(ctx, t.repoRoot, &repoTools.ApplyPatchParams{Patch: args.Patch})
			if res.OK {
				t.applied = true
			}
			if t.trace != nil {
				t.trace(summarizeResult("apply_patch", res))
			}
			return res.JSON(), nil
		})
		if err != nil {
			return nil, err
		}
		applyPatchTool = toolImpl
	}

	runCmdTool, err := toolutils.InferTool("run_cmd", "Run an allowed command by name", func(ctx context.Context, args *runCmdArgs) (string, error) {
		if args != nil && args.Name == "test" && t.requirePatch && !t.applied {
			return repoTools.Err("apply_patch required before running test").JSON(), nil
		}
		if t.trace != nil {
			t.trace(fmt.Sprintf("【run_cmd】name=%q", args.Name))
		}
		res := repoTools.RunCmd(ctx, t.repoRoot, t.allowed, &repoTools.RunCmdParams{Name: args.Name})
		if t.trace != nil {
			t.trace(summarizeResult("run_cmd", res))
		}
		return res.JSON(), nil
	})
	if err != nil {
		return nil, err
	}

	gitDiffTool, err := toolutils.InferTool("git_diff", "Get current git diff", func(ctx context.Context, _ *struct{}) (string, error) {
		if t.trace != nil {
			t.trace("【git_diff】")
		}
		res := repoTools.GitDiff(ctx, t.repoRoot)
		if t.trace != nil {
			t.trace(summarizeResult("git_diff", res))
		}
		return res.JSON(), nil
	})
	if err != nil {
		return nil, err
	}

	tools := []tool.BaseTool{repoTreeTool, searchTool, openFileTool}
	if opts.includeApplyPatch {
		tools = append(tools, applyPatchTool)
	}
	tools = append(tools, runCmdTool, gitDiffTool)
	return tools, nil
}

func stripLineNumbers(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "→"); idx >= 0 {
			lines[i] = line[idx+len("→"):]
		}
	}
	return strings.Join(lines, "\n")
}

func summarizeResult(name string, res repoTools.Result) string {
	if !res.OK {
		if strings.TrimSpace(res.Error) == "" {
			return fmt.Sprintf("【%s】error", name)
		}
		return fmt.Sprintf("【%s】error=%s", name, res.Error)
	}
	switch name {
	case "repo_tree":
		if data, ok := res.Data.(repoTools.RepoTreeData); ok {
			return fmt.Sprintf("【%s】entries=%d", name, len(data.Entries))
		}
		if dataPtr, ok := res.Data.(*repoTools.RepoTreeData); ok && dataPtr != nil {
			return fmt.Sprintf("【%s】entries=%d", name, len(dataPtr.Entries))
		}
	case "search_code":
		if data, ok := res.Data.(repoTools.SearchCodeData); ok {
			return fmt.Sprintf("【%s】matches=%d", name, len(data.Matches))
		}
		if dataPtr, ok := res.Data.(*repoTools.SearchCodeData); ok && dataPtr != nil {
			return fmt.Sprintf("【%s】matches=%d", name, len(dataPtr.Matches))
		}
	case "open_file":
		if data, ok := res.Data.(repoTools.OpenFileData); ok {
			return fmt.Sprintf("【%s】path=%s line_count=%d", name, data.Path, data.LineCount)
		}
		if dataPtr, ok := res.Data.(*repoTools.OpenFileData); ok && dataPtr != nil {
			return fmt.Sprintf("【%s】path=%s line_count=%d", name, dataPtr.Path, dataPtr.LineCount)
		}
	case "apply_patch":
		if data, ok := res.Data.(repoTools.ApplyPatchData); ok {
			return fmt.Sprintf("【%s】phase=%s", name, data.Phase)
		}
		if dataPtr, ok := res.Data.(*repoTools.ApplyPatchData); ok && dataPtr != nil {
			return fmt.Sprintf("【%s】phase=%s", name, dataPtr.Phase)
		}
	case "run_cmd":
		if data, ok := res.Data.(repoTools.RunCmdData); ok {
			return fmt.Sprintf("【%s】exit_code=%d", name, data.ExitCode)
		}
		if dataPtr, ok := res.Data.(*repoTools.RunCmdData); ok && dataPtr != nil {
			return fmt.Sprintf("【%s】exit_code=%d", name, dataPtr.ExitCode)
		}
	case "git_diff":
		if data, ok := res.Data.(repoTools.GitDiffData); ok {
			return fmt.Sprintf("【%s】bytes=%d", name, len(data.Diff))
		}
		if dataPtr, ok := res.Data.(*repoTools.GitDiffData); ok && dataPtr != nil {
			return fmt.Sprintf("【%s】bytes=%d", name, len(dataPtr.Diff))
		}
	}
	return fmt.Sprintf("【%s】ok", name)
}
