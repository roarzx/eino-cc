package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	patch := normalizePatch(params.Patch)
	patch = filterPatchByRepo(patch, repoRoot)
	if strings.TrimSpace(patch) == "" {
		return Err("patch does not match repo files")
	}
	if !strings.Contains(patch, "diff --git ") {
		return Err("patch must include diff --git")
	}

	cmd := exec.CommandContext(ctx, "git", "apply", "--whitespace=nowarn")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)

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
		if applied, err := applySimplePatch(patch, repoRoot); applied && err == nil {
			return OK(ApplyPatchData{ExitCode: 0, Stdout: "fallback applied", Stderr: ""})
		}
		return Result{OK: false, Error: "git apply failed", Data: data}
	}
	return OK(data)
}

func normalizePatch(text string) string {
	s := strings.TrimSpace(text)
	s = strings.TrimPrefix(s, "```diff")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		if unquoted, err := strconv.Unquote(s); err == nil {
			s = unquoted
		}
	}
	if strings.Contains(s, "\\n") {
		s = strings.ReplaceAll(s, "\\n", "\n")
		s = strings.ReplaceAll(s, "\\t", "\t")
		s = strings.ReplaceAll(s, "\\\"", "\"")
		s = strings.ReplaceAll(s, "\\\\", "\\")
	}
	s = decodeUnicodeEscapes(s)
	if idx := strings.Index(s, "diff --git "); idx >= 0 {
		s = s[idx:]
	}
	return strings.TrimSpace(s)
}

func decodeUnicodeEscapes(text string) string {
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		if i+5 < len(text) && text[i] == '\\' && text[i+1] == 'u' {
			hex := text[i+2 : i+6]
			if r, err := strconv.ParseInt(hex, 16, 32); err == nil {
				b.WriteRune(rune(r))
				i += 5
				continue
			}
		}
		b.WriteByte(text[i])
	}
	return b.String()
}

func filterPatchByRepo(patch string, repoRoot string) string {
	lines := strings.Split(patch, "\n")
	blocks := make([]string, 0)
	for i := 0; i < len(lines); {
		if !strings.HasPrefix(lines[i], "diff --git ") {
			i++
			continue
		}
		start := i
		i++
		for i < len(lines) && !strings.HasPrefix(lines[i], "diff --git ") {
			i++
		}
		block := strings.Join(lines[start:i], "\n")
		if diffTouchesRepo(block, repoRoot) {
			blocks = append(blocks, block)
		}
	}
	return strings.TrimSpace(strings.Join(blocks, "\n"))
}

func diffTouchesRepo(block string, repoRoot string) bool {
	lines := strings.Split(block, "\n")
	if len(lines) == 0 {
		return false
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 4 {
		return true
	}
	base := filepath.Base(repoRoot)
	aPath := normalizeRepoPath(strings.TrimPrefix(fields[2], "a/"), base)
	bPath := normalizeRepoPath(strings.TrimPrefix(fields[3], "b/"), base)
	if aPath == "dev/null" || bPath == "dev/null" {
		return true
	}
	if fileExists(filepath.Join(repoRoot, aPath)) || fileExists(filepath.Join(repoRoot, bPath)) {
		return true
	}
	return false
}

func normalizeRepoPath(path string, base string) string {
	if base == "" {
		return path
	}
	prefix := base + string(filepath.Separator)
	if strings.HasPrefix(path, prefix) {
		return strings.TrimPrefix(path, prefix)
	}
	if strings.HasPrefix(path, base+"/") {
		return strings.TrimPrefix(path, base+"/")
	}
	return path
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func applySimplePatch(patch string, repoRoot string) (bool, error) {
	lines := strings.Split(patch, "\n")
	var aPath, bPath string
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				aPath = strings.TrimPrefix(fields[2], "a/")
				bPath = strings.TrimPrefix(fields[3], "b/")
			}
			break
		}
	}
	if aPath == "" && bPath == "" {
		return false, nil
	}
	base := filepath.Base(repoRoot)
	aPath = normalizeRepoPath(aPath, base)
	bPath = normalizeRepoPath(bPath, base)
	target := bPath
	if target == "" || target == "dev/null" {
		target = aPath
	}
	if target == "" || target == "dev/null" {
		return false, nil
	}
	abs := filepath.Join(repoRoot, target)
	content, err := os.ReadFile(abs)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return false, err
	}
	fileLines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	oldStart := 0
	oldCount := 0
	newStart := 0
	newCount := 0
	oldLine := ""
	newLine := ""
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "@@") {
			oldCount = 1
			newCount = 1
			if _, err := fmt.Sscanf(line, "@@ -%d,%d +%d,%d @@", &oldStart, &oldCount, &newStart, &newCount); err != nil {
				if _, err2 := fmt.Sscanf(line, "@@ -%d +%d @@", &oldStart, &newStart); err2 != nil {
					_, _ = fmt.Sscanf(line, "@@ -%d,%d +%d @@", &oldStart, &oldCount, &newStart)
				}
			}
			for j := i + 1; j < len(lines); j++ {
				h := lines[j]
				if strings.HasPrefix(h, "@@") || strings.HasPrefix(h, "diff --git ") {
					break
				}
				if strings.HasPrefix(h, "-") && !strings.HasPrefix(h, "---") {
					oldLine = strings.TrimPrefix(h, "-")
				}
				if strings.HasPrefix(h, "+") && !strings.HasPrefix(h, "+++") {
					newLine = strings.TrimPrefix(h, "+")
				}
			}
			break
		}
	}
	if oldLine == "" && newLine == "" {
		return false, nil
	}
	applied := false
	if oldStart > 0 && oldStart <= len(fileLines) {
		if fileLines[oldStart-1] == oldLine {
			fileLines[oldStart-1] = newLine
			applied = true
		}
	}
	if !applied && oldLine != "" {
		for i, line := range fileLines {
			if line == oldLine {
				fileLines[i] = newLine
				applied = true
				break
			}
		}
	}
	if !applied {
		return false, nil
	}
	output := strings.Join(fileLines, "\n")
	return true, os.WriteFile(abs, []byte(output), info.Mode())
}
