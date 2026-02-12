package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"eino-cc/internal/config"
	repoTools "eino-cc/internal/tools"
)

type Session struct {
	RepoRoot     string
	LastGoal     string
	LastFailure  string
	PendingPatch string
}

type intentKind string

const (
	intentTask     intentKind = "task"
	intentDiff     intentKind = "diff"
	intentStatus   intentKind = "status"
	intentRollback intentKind = "rollback"
	intentApply    intentKind = "apply"
	intentRetry    intentKind = "retry"
	intentExit     intentKind = "exit"
)

func RunInteractive(ctx context.Context, cfg *config.Config, repoRoot string, initialGoal string) error {
	session := &Session{RepoRoot: repoRoot}
	allowed := allowedCommands(cfg)

	handleInput := func(text string) bool {
		text = strings.TrimSpace(text)
		if text == "" {
			return false
		}

		if strings.TrimSpace(session.PendingPatch) != "" {
			if isAffirmative(text) {
				if err := applyPendingPatch(ctx, allowed, session); err != nil {
					fmt.Println(err.Error())
				}
				return false
			}
			if isNegative(text) {
				session.PendingPatch = ""
				fmt.Println("已取消应用补丁。")
				return false
			}
		}

		if updated := maybeSetRepoFromText(ctx, text, session); updated {
			fmt.Printf("已切换仓库：%s\n", session.RepoRoot)
			if isRepoSelectionOnly(text) {
				return false
			}
		}

		switch classifyIntent(text) {
		case intentExit:
			return true
		case intentDiff:
			showDiff(ctx, session)
		case intentStatus:
			showStatus(ctx, session)
		case intentRollback:
			rollbackWorktree(ctx, session)
		case intentApply:
			if strings.TrimSpace(session.PendingPatch) == "" {
				fmt.Println("当前没有待应用的补丁。")
				return false
			}
			if err := applyPendingPatch(ctx, allowed, session); err != nil {
				fmt.Println(err.Error())
			}
		case intentRetry:
			if strings.TrimSpace(session.LastGoal) == "" {
				fmt.Println("没有可重试的目标。")
				return false
			}
			if err := runTask(ctx, cfg, session.LastGoal, session); err != nil {
				fmt.Println(err.Error())
			}
		default:
			if err := runTask(ctx, cfg, text, session); err != nil {
				fmt.Println(err.Error())
			}
		}
		return false
	}

	fmt.Println("进入交互模式：直接输入自然语言目标（例如：修复 calc.Add 让测试通过）。输入“退出”结束。")

	if strings.TrimSpace(initialGoal) != "" {
		if quit := handleInput(initialGoal); quit {
			return nil
		}
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		if quit := handleInput(scanner.Text()); quit {
			return nil
		}
	}
	return scanner.Err()
}

func allowedCommands(cfg *config.Config) map[string]string {
	allowed := map[string]string{}
	if cfg != nil && cfg.Commands.Test != "" {
		allowed["test"] = cfg.Commands.Test
	}
	if cfg != nil && strings.TrimSpace(cfg.Commands.Fmt) != "" {
		allowed["fmt"] = cfg.Commands.Fmt
	}
	if allowed["test"] == "" {
		allowed["test"] = "go test ./..."
	}
	return allowed
}

func runTask(ctx context.Context, cfg *config.Config, goal string, session *Session) error {
	session.LastGoal = goal
	fmt.Printf("我理解你的意思是：%s\n", goal)

	if strings.TrimSpace(session.RepoRoot) == "" {
		if !maybeSetRepoFromText(ctx, goal, session) {
			fmt.Println("请先告诉我仓库目录，例如：使用 ./test-repo 或 在 /path/to/repo 修复某函数。")
			return nil
		}
		fmt.Printf("已切换仓库：%s\n", session.RepoRoot)
	}

	r := NewRunner(cfg, session.RepoRoot)
	trace := func(line string) {
		if strings.TrimSpace(line) == "" {
			return
		}
		fmt.Println(line)
	}

	patch, err := r.Propose(ctx, goal, session.LastFailure, trace)
	if err != nil {
		return err
	}
	if strings.TrimSpace(patch) == "" {
		fmt.Println("我没有生成可应用的补丁。你可以让我读取更多文件，或更具体描述要改的函数/行为。")
		return nil
	}

	session.PendingPatch = patch
	fmt.Println("我生成了补丁预览如下：")
	fmt.Println(patch)
	fmt.Println("是否应用？(y/N)")
	return nil
}

func applyPendingPatch(ctx context.Context, allowed map[string]string, session *Session) error {
	patch := strings.TrimSpace(session.PendingPatch)
	if patch == "" {
		return nil
	}
	if strings.TrimSpace(session.RepoRoot) == "" {
		return fmt.Errorf("尚未设置仓库目录，无法应用补丁")
	}

	applyRes := repoTools.ApplyPatch(ctx, session.RepoRoot, &repoTools.ApplyPatchParams{Patch: patch})
	if !applyRes.OK {
		session.LastFailure = applyRes.JSON()
		return fmt.Errorf("应用补丁失败：%s", session.LastFailure)
	}
	session.PendingPatch = ""

	fmt.Println("已应用补丁。现在运行测试…")
	testRes := repoTools.RunCmd(ctx, session.RepoRoot, allowed, &repoTools.RunCmdParams{Name: "test"})
	if testRes.OK {
		session.LastFailure = ""
		fmt.Println("测试通过。当前变更：")
		showDiff(ctx, session)
		return nil
	}
	session.LastFailure = formatTestFailure(testRes)
	return fmt.Errorf("测试失败：\n%s\n你可以说：继续重试 / 回滚一下 / 看下 diff", session.LastFailure)
}

func showDiff(ctx context.Context, session *Session) {
	if strings.TrimSpace(session.RepoRoot) == "" {
		fmt.Println("尚未设置仓库目录，无法查看 diff。")
		return
	}
	res := repoTools.GitDiff(ctx, session.RepoRoot)
	if !res.OK {
		fmt.Println(res.JSON())
		return
	}
	if data, ok := res.Data.(repoTools.GitDiffData); ok {
		if strings.TrimSpace(data.Diff) == "" {
			fmt.Println("无未提交变更。")
			return
		}
		fmt.Println(data.Diff)
		return
	}
	if dataPtr, ok := res.Data.(*repoTools.GitDiffData); ok && dataPtr != nil {
		if strings.TrimSpace(dataPtr.Diff) == "" {
			fmt.Println("无未提交变更。")
			return
		}
		fmt.Println(dataPtr.Diff)
		return
	}
	fmt.Println(res.JSON())
}

func showStatus(ctx context.Context, session *Session) {
	if strings.TrimSpace(session.RepoRoot) == "" {
		fmt.Println("尚未设置仓库目录，无法查看状态。")
		return
	}
	out, err := repoTools.RunCommand(ctx, session.RepoRoot, "git", "status", "--porcelain")
	if err != nil {
		fmt.Printf("状态获取失败：%v\n", err)
		return
	}
	dirty := strings.TrimSpace(out.Stdout) != ""
	fmt.Printf("worktree_dirty=%v pending_patch=%v\n", dirty, strings.TrimSpace(session.PendingPatch) != "")
	if strings.TrimSpace(session.LastFailure) != "" {
		fmt.Println("上一次失败摘要：")
		fmt.Println(session.LastFailure)
	}
	if dirty {
		fmt.Println(out.Stdout)
	}
}

func rollbackWorktree(ctx context.Context, session *Session) {
	if strings.TrimSpace(session.RepoRoot) == "" {
		fmt.Println("尚未设置仓库目录，无法回滚。")
		return
	}
	session.PendingPatch = ""
	out, err := repoTools.RunCommand(ctx, session.RepoRoot, "git", "restore", ".")
	if err == nil && out.ExitCode == 0 {
		fmt.Println("已回滚工作区。")
		return
	}
	out2, err2 := repoTools.RunCommand(ctx, session.RepoRoot, "git", "checkout", "--", ".")
	if err2 == nil && out2.ExitCode == 0 {
		fmt.Println("已回滚工作区。")
		return
	}
	fmt.Println("回滚失败。")
}

func maybeSetRepoFromText(ctx context.Context, text string, session *Session) bool {
	path := extractRepoPath(text)
	if path == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	out, err := repoTools.RunCommand(ctx, abs, "git", "rev-parse", "--is-inside-work-tree")
	if err != nil || out.ExitCode != 0 {
		return false
	}
	if session.RepoRoot != abs {
		session.RepoRoot = abs
		session.PendingPatch = ""
		session.LastFailure = ""
	}
	return true
}

func extractRepoPath(text string) string {
	clean := strings.NewReplacer("，", " ", "。", " ", "：", " ", "；", " ", "、", " ", "\t", " ").Replace(text)
	parts := strings.Fields(clean)
	for _, part := range parts {
		token := strings.Trim(part, "`\"'")
		if token == "" {
			continue
		}
		if strings.Contains(token, "repo-root=") {
			token = strings.TrimPrefix(token, "repo-root=")
		}
		if strings.Contains(token, "repo=") {
			token = strings.TrimPrefix(token, "repo=")
		}
		if strings.Contains(token, "目录=") {
			token = strings.TrimPrefix(token, "目录=")
		}
		if strings.Contains(token, "路径=") {
			token = strings.TrimPrefix(token, "路径=")
		}
		token = strings.Trim(token, "`\"'")
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "./") || strings.HasPrefix(token, "../") || strings.HasPrefix(token, "/") || strings.HasPrefix(token, "~") || strings.Contains(token, "/") {
			return token
		}
	}
	return ""
}

func classifyIntent(text string) intentKind {
	s := strings.ToLower(strings.TrimSpace(text))
	if s == "" {
		return intentTask
	}
	for _, kw := range []string{"退出", "结束", "exit", "quit"} {
		if strings.Contains(s, kw) {
			return intentExit
		}
	}
	for _, kw := range []string{"diff", "改动", "变更", "改了什么"} {
		if strings.Contains(s, kw) {
			return intentDiff
		}
	}
	for _, kw := range []string{"状态", "进度", "现在怎么样", "status"} {
		if strings.Contains(s, kw) {
			return intentStatus
		}
	}
	for _, kw := range []string{"回滚", "撤销", "恢复", "还原", "rollback"} {
		if strings.Contains(s, kw) {
			return intentRollback
		}
	}
	for _, kw := range []string{"应用", "apply", "确认", "就这么改"} {
		if strings.Contains(s, kw) {
			return intentApply
		}
	}
	for _, kw := range []string{"重试", "再试", "继续", "retry"} {
		if strings.Contains(s, kw) {
			return intentRetry
		}
	}
	return intentTask
}

func isAffirmative(text string) bool {
	s := strings.ToLower(strings.TrimSpace(text))
	if s == "y" || s == "yes" || s == "ok" {
		return true
	}
	for _, kw := range []string{"是", "好", "确认", "应用", "继续"} {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func isNegative(text string) bool {
	s := strings.ToLower(strings.TrimSpace(text))
	if s == "n" || s == "no" {
		return true
	}
	for _, kw := range []string{"否", "不要", "取消", "先别"} {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func isRepoSelectionOnly(text string) bool {
	if extractRepoPath(text) == "" {
		return false
	}
	lower := strings.ToLower(text)
	keywords := []string{"使用", "切换", "目录", "仓库", "repo", "在"}
	seen := false
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			seen = true
			break
		}
	}
	if !seen {
		return false
	}
	actionKeywords := []string{"修复", "修改", "实现", "添加", "删除", "重构", "测试", "运行", "生成", "优化", "分析", "解释"}
	for _, kw := range actionKeywords {
		if strings.Contains(lower, kw) {
			return false
		}
	}
	return true
}
