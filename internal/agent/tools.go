package agent

import (
	"context"
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
	Name string `json:"name" jsonschema:"description=command name; allowed: test"`
}

func (t *repoToolset) Tools(ctx context.Context) ([]tool.BaseTool, error) {
	repoTreeTool, err := toolutils.InferTool("repo_tree", "List files in the git repository", func(ctx context.Context, args *repoTreeArgs) (string, error) {
		if t.openedNonTestFile && !t.applied {
			return repoTools.Err("apply_patch required after open_file").JSON(), nil
		}
		res := repoTools.RepoTree(ctx, t.repoRoot, &repoTools.RepoTreeParams{MaxEntries: args.MaxEntries})
		return res.JSON(), nil
	})
	if err != nil {
		return nil, err
	}

	searchTool, err := toolutils.InferTool("search_code", "Search code using ripgrep", func(ctx context.Context, args *searchCodeArgs) (string, error) {
		if t.openedNonTestFile && !t.applied {
			return repoTools.Err("apply_patch required after open_file").JSON(), nil
		}
		res := repoTools.SearchCode(ctx, t.repoRoot, &repoTools.SearchCodeParams{Query: args.Query, MaxResults: args.MaxResults})
		return res.JSON(), nil
	})
	if err != nil {
		return nil, err
	}

	openFileTool, err := toolutils.InferTool("open_file", "Open and read a file", func(ctx context.Context, args *openFileArgs) (string, error) {
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
		return res.JSON(), nil
	})
	if err != nil {
		return nil, err
	}

	applyPatchTool, err := toolutils.InferTool("apply_patch", "Apply a unified diff patch with git apply", func(ctx context.Context, args *applyPatchArgs) (string, error) {
		res := repoTools.ApplyPatch(ctx, t.repoRoot, &repoTools.ApplyPatchParams{Patch: args.Patch})
		if res.OK {
			t.applied = true
		}
		return res.JSON(), nil
	})
	if err != nil {
		return nil, err
	}

	runCmdTool, err := toolutils.InferTool("run_cmd", "Run an allowed command by name", func(ctx context.Context, args *runCmdArgs) (string, error) {
		if args != nil && args.Name == "test" && t.requirePatch && !t.applied {
			return repoTools.Err("apply_patch required before running test").JSON(), nil
		}
		res := repoTools.RunCmd(ctx, t.repoRoot, t.allowed, &repoTools.RunCmdParams{Name: args.Name})
		return res.JSON(), nil
	})
	if err != nil {
		return nil, err
	}

	gitDiffTool, err := toolutils.InferTool("git_diff", "Get current git diff", func(ctx context.Context, _ *struct{}) (string, error) {
		res := repoTools.GitDiff(ctx, t.repoRoot)
		return res.JSON(), nil
	})
	if err != nil {
		return nil, err
	}

	return []tool.BaseTool{repoTreeTool, searchTool, openFileTool, applyPatchTool, runCmdTool, gitDiffTool}, nil
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
