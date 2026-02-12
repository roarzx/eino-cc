package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"eino-cc/internal/config"
	repoTools "eino-cc/internal/tools"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const appVersion = "dev"

type proposeResultMsg struct {
	goal  string
	patch string
	logs  []string
	err   error
}

type applyResultMsg struct {
	ok      bool
	failure string
	diff    string
	err     error
}

type infoMsg string
type initialInputMsg string

type tuiModel struct {
	ctx context.Context
	cfg *config.Config

	session Session
	state   *recentState

	startedAt time.Time

	width  int
	height int

	view     viewport.Model
	input    textinput.Model
	log      string
	logLines int

	showHelp    bool
	confirmMode bool

	confirmPrompt string

	statusLine string

	initialGoal string
}

func (m tuiModel) contentWidth() int {
	w := m.width
	if w <= 0 {
		w = 90
	}
	if w < 40 {
		w = 40
	}
	return w
}

func (m tuiModel) userStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
}

func (m tuiModel) assistantStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
}

func (m tuiModel) toolStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
}

func (m tuiModel) metaStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
}

func (m *tuiModel) appendUser(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if strings.TrimSpace(m.log) != "" {
		m.appendLine("")
	}
	m.appendLine(m.userStyle().Render("❯ " + text))
}

func (m *tuiModel) appendMeta(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	m.appendLine(m.metaStyle().Render(text))
}

func (m *tuiModel) appendToolBlock(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	w := m.contentWidth()
	style := m.toolStyle().Width(w).MaxWidth(w)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			m.appendLine("")
			continue
		}
		m.appendLine(style.Render("  " + line))
	}
}

func (m *tuiModel) appendAssistantBlock(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	w := m.contentWidth()
	style := lipgloss.NewStyle().Width(w).MaxWidth(w)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			m.appendLine("")
			continue
		}
		if i == 0 {
			m.appendLine(m.assistantStyle().Render("● ") + style.Render(line))
			continue
		}
		m.appendLine(style.Render("  " + line))
	}
}

func isToolLikeLine(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "【") {
		return true
	}
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
		return true
	}
	return false
}

func RunTUI(ctx context.Context, cfg *config.Config, repoRoot string, initialGoal string) error {
	m := newTUIModel(ctx, cfg, repoRoot, initialGoal)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func newTUIModel(ctx context.Context, cfg *config.Config, repoRoot string, initialGoal string) tuiModel {
	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.Placeholder = "输入自然语言目标，例如：使用 ./test-repo；修复 calc.Add 让测试通过"
	ti.Focus()
	ti.CharLimit = 0

	vp := viewport.New(0, 0)
	vp.YPosition = 0
	vp.SetContent("")

	st := loadRecentState()
	session := Session{RepoRoot: repoRoot}

	m := tuiModel{
		ctx:         ctx,
		cfg:         cfg,
		session:     session,
		state:       st,
		startedAt:   time.Now(),
		view:        vp,
		input:       ti,
		log:         "",
		logLines:    0,
		initialGoal: strings.TrimSpace(initialGoal),
	}
	m.appendMeta(m.renderSessionStartLine())
	m.appendMeta(m.renderTryLine())
	m.updateStatusLine()
	return m
}

func (m tuiModel) Init() tea.Cmd {
	if strings.TrimSpace(m.initialGoal) == "" {
		return nil
	}
	goal := m.initialGoal
	return func() tea.Msg { return initialInputMsg(goal) }
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case initialInputMsg:
		return m.handleUserInput(string(msg))
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.showHelp = false
			m.confirmMode = false
			m.confirmPrompt = ""
			m.updateStatusLine()
			m.resize()
			return m, nil
		case "?":
			m.showHelp = !m.showHelp
			m.updateStatusLine()
			m.resize()
			return m, nil
		case "enter":
			text := strings.TrimSpace(m.input.Value())
			m.input.SetValue("")
			if text == "" {
				return m, nil
			}
			if m.confirmMode {
				return m.handleConfirmInput(text)
			}
			return m.handleUserInput(text)
		}
	case proposeResultMsg:
		var assistantBuf strings.Builder
		var toolBuf strings.Builder
		flushAssistant := func() {
			s := strings.TrimSpace(assistantBuf.String())
			if s == "" {
				assistantBuf.Reset()
				return
			}
			m.appendAssistantBlock(s)
			assistantBuf.Reset()
		}
		flushTool := func() {
			s := strings.TrimSpace(toolBuf.String())
			if s == "" {
				toolBuf.Reset()
				return
			}
			m.appendToolBlock(s)
			toolBuf.Reset()
		}
		appendAssistantLine := func(line string) {
			line = strings.TrimRight(line, "\r")
			if strings.TrimSpace(line) == "" {
				assistantBuf.WriteString("\n")
				return
			}
			assistantBuf.WriteString(line)
			assistantBuf.WriteString("\n")
		}
		appendToolLine := func(line string) {
			line = strings.TrimRight(line, "\r")
			if strings.TrimSpace(line) == "" {
				toolBuf.WriteString("\n")
				return
			}
			toolBuf.WriteString(line)
			toolBuf.WriteString("\n")
		}

		for _, chunk := range msg.logs {
			for _, line := range strings.Split(chunk, "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "⎿") {
					flushAssistant()
					flushTool()
					m.appendMeta(line)
					continue
				}
				if isToolLikeLine(line) {
					flushAssistant()
					appendToolLine(line)
					continue
				}
				flushTool()
				appendAssistantLine(line)
			}
		}
		flushTool()
		flushAssistant()
		if msg.err != nil {
			m.appendMeta(fmt.Sprintf("⎿ propose error: %s", msg.err.Error()))
			m.session.LastFailure = msg.err.Error()
			m.updateStatusLine()
			m.resize()
			return m, nil
		}
		m.session.LastGoal = msg.goal
		if strings.TrimSpace(msg.patch) == "" {
			m.appendAssistantBlock("我没有生成可应用的补丁。你可以让我读取更多文件，或更具体描述要改的函数/行为。")
			m.updateStatusLine()
			m.resize()
			return m, nil
		}
		m.session.PendingPatch = msg.patch
		m.confirmMode = true
		m.confirmPrompt = "是否应用补丁？(y/N)"
		m.appendAssistantBlock("我生成了补丁预览如下：")
		m.appendToolBlock(msg.patch)
		m.appendMeta(m.confirmPrompt)
		m.updateStatusLine()
		m.noteActivity()
		m.resize()
		return m, nil
	case applyResultMsg:
		if msg.err != nil {
			m.appendAssistantBlock(fmt.Sprintf("应用失败：%s", msg.err.Error()))
			m.session.LastFailure = msg.err.Error()
			m.updateStatusLine()
			m.resize()
			return m, nil
		}
		if msg.ok {
			m.session.LastFailure = ""
			m.appendAssistantBlock("测试通过。当前变更：")
			if strings.TrimSpace(msg.diff) == "" {
				m.appendToolBlock("无未提交变更。")
			} else {
				m.appendToolBlock(msg.diff)
			}
			m.updateStatusLine()
			m.noteActivity()
			m.resize()
			return m, nil
		}
		m.session.LastFailure = msg.failure
		m.appendAssistantBlock("测试失败：")
		m.appendToolBlock(msg.failure)
		m.appendMeta("你可以说：继续重试 / 回滚一下 / 看下 diff")
		m.updateStatusLine()
		m.noteActivity()
		m.resize()
		return m, nil
	case infoMsg:
		m.appendToolBlock(string(msg))
		m.updateStatusLine()
		m.resize()
		return m, nil
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.view, cmd = m.view.Update(msg)
	cmds = append(cmds, cmd)

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m tuiModel) View() string {
	if m.width == 0 || m.height == 0 {
		return m.renderHeader() + "\n" + m.renderPromptArea()
	}
	blocks := []string{m.renderHeader(), m.view.View()}
	if m.showHelp {
		blocks = append(blocks, m.renderHelp())
	}
	if m.confirmMode && strings.TrimSpace(m.confirmPrompt) != "" {
		blocks = append(blocks, m.renderConfirm())
	}
	blocks = append(blocks, m.renderPromptArea())
	return strings.Join(blocks, "\n")
}

func (m *tuiModel) handleUserInput(text string) (tea.Model, tea.Cmd) {
	m.appendUser(text)
	if updated := maybeSetRepoFromText(m.ctx, text, &m.session); updated {
		m.appendMeta(fmt.Sprintf("⎿ Repo switched: %s", m.session.RepoRoot))
		m.noteActivity()
		if isRepoSelectionOnly(text) {
			m.updateStatusLine()
			m.resize()
			return m, nil
		}
	}
	switch classifyIntent(text) {
	case intentExit:
		return m, tea.Quit
	case intentStatus:
		return m, m.statusCmd()
	case intentDiff:
		return m, m.diffCmd()
	case intentRollback:
		return m, m.rollbackCmd()
	case intentRetry:
		if strings.TrimSpace(m.session.LastGoal) == "" {
			m.appendAssistantBlock("没有可重试的目标。")
			m.updateStatusLine()
			m.resize()
			return m, nil
		}
		return m, m.proposeCmd(m.session.LastGoal)
	case intentApply:
		if strings.TrimSpace(m.session.PendingPatch) == "" {
			m.appendAssistantBlock("当前没有待应用的补丁。")
			m.updateStatusLine()
			m.resize()
			return m, nil
		}
		m.confirmMode = true
		m.confirmPrompt = "是否应用补丁？(y/N)"
		m.appendMeta(m.confirmPrompt)
		m.updateStatusLine()
		m.resize()
		return m, nil
	default:
		if strings.TrimSpace(m.session.RepoRoot) == "" {
			m.appendAssistantBlock("请先告诉我仓库目录，例如：使用 ./test-repo 或 在 /path/to/repo 修复某函数。")
			m.updateStatusLine()
			m.resize()
			return m, nil
		}
		return m, m.proposeCmd(text)
	}
}

func (m *tuiModel) handleConfirmInput(text string) (tea.Model, tea.Cmd) {
	m.appendUser(text)
	if isNegative(text) {
		m.session.PendingPatch = ""
		m.confirmMode = false
		m.confirmPrompt = ""
		m.appendAssistantBlock("已取消应用补丁。")
		m.updateStatusLine()
		m.resize()
		return m, nil
	}
	if !isAffirmative(text) {
		m.appendMeta(m.confirmPrompt)
		m.updateStatusLine()
		m.resize()
		return m, nil
	}
	if strings.TrimSpace(m.session.RepoRoot) == "" {
		m.confirmMode = false
		m.confirmPrompt = ""
		m.appendAssistantBlock("尚未设置仓库目录，无法应用补丁。")
		m.updateStatusLine()
		m.resize()
		return m, nil
	}
	patch := m.session.PendingPatch
	m.session.PendingPatch = ""
	m.confirmMode = false
	m.confirmPrompt = ""
	m.appendMeta("⎿ Applying patch and running tests…")
	m.updateStatusLine()
	m.resize()
	return m, m.applyCmd(patch)
}

func (m *tuiModel) proposeCmd(goal string) tea.Cmd {
	repoRoot := m.session.RepoRoot
	lastFailure := m.session.LastFailure
	cfg := m.cfg
	ctx := m.ctx
	m.appendMeta("⎿ Thinking…")
	m.updateStatusLine()
	m.resize()
	return func() tea.Msg {
		lines := make([]string, 0, 64)
		emit := func(s string) {
			s = strings.TrimSpace(s)
			if s == "" {
				return
			}
			lines = append(lines, s)
		}
		r := NewRunner(cfg, repoRoot)
		patch, err := r.Propose(ctx, goal, lastFailure, emit)
		return proposeResultMsg{goal: goal, patch: patch, logs: lines, err: err}
	}
}

func (m *tuiModel) applyCmd(patch string) tea.Cmd {
	repoRoot := m.session.RepoRoot
	allowed := allowedCommands(m.cfg)
	ctx := m.ctx
	return func() tea.Msg {
		applyRes := repoTools.ApplyPatch(ctx, repoRoot, &repoTools.ApplyPatchParams{Patch: patch})
		if !applyRes.OK {
			return applyResultMsg{err: fmt.Errorf("%s", applyRes.JSON())}
		}
		testRes := repoTools.RunCmd(ctx, repoRoot, allowed, &repoTools.RunCmdParams{Name: "test"})
		if testRes.OK {
			diffRes := repoTools.GitDiff(ctx, repoRoot)
			if diffRes.OK {
				if data, ok := diffRes.Data.(repoTools.GitDiffData); ok {
					return applyResultMsg{ok: true, diff: data.Diff}
				}
				if dataPtr, ok := diffRes.Data.(*repoTools.GitDiffData); ok && dataPtr != nil {
					return applyResultMsg{ok: true, diff: dataPtr.Diff}
				}
			}
			return applyResultMsg{ok: true}
		}
		return applyResultMsg{ok: false, failure: formatTestFailure(testRes)}
	}
}

func (m *tuiModel) diffCmd() tea.Cmd {
	repoRoot := m.session.RepoRoot
	ctx := m.ctx
	if strings.TrimSpace(repoRoot) == "" {
		return func() tea.Msg { return infoMsg("尚未设置仓库目录，无法查看 diff。") }
	}
	return func() tea.Msg {
		res := repoTools.GitDiff(ctx, repoRoot)
		if !res.OK {
			return infoMsg(res.JSON())
		}
		if data, ok := res.Data.(repoTools.GitDiffData); ok {
			if strings.TrimSpace(data.Diff) == "" {
				return infoMsg("无未提交变更。")
			}
			return infoMsg(data.Diff)
		}
		if dataPtr, ok := res.Data.(*repoTools.GitDiffData); ok && dataPtr != nil {
			if strings.TrimSpace(dataPtr.Diff) == "" {
				return infoMsg("无未提交变更。")
			}
			return infoMsg(dataPtr.Diff)
		}
		return infoMsg(res.JSON())
	}
}

func (m *tuiModel) statusCmd() tea.Cmd {
	repoRoot := m.session.RepoRoot
	ctx := m.ctx
	hasPending := strings.TrimSpace(m.session.PendingPatch) != ""
	if strings.TrimSpace(repoRoot) == "" {
		return func() tea.Msg { return infoMsg("尚未设置仓库目录，无法查看状态。") }
	}
	return func() tea.Msg {
		out, err := repoTools.RunCommand(ctx, repoRoot, "git", "status", "--porcelain")
		if err != nil {
			return infoMsg(fmt.Sprintf("状态获取失败：%v", err))
		}
		dirty := strings.TrimSpace(out.Stdout) != ""
		header := fmt.Sprintf("worktree_dirty=%v pending_patch=%v", dirty, hasPending)
		if !dirty {
			return infoMsg(header)
		}
		return infoMsg(header + "\n" + out.Stdout)
	}
}

func (m *tuiModel) rollbackCmd() tea.Cmd {
	repoRoot := m.session.RepoRoot
	ctx := m.ctx
	if strings.TrimSpace(repoRoot) == "" {
		return func() tea.Msg { return infoMsg("尚未设置仓库目录，无法回滚。") }
	}
	m.session.PendingPatch = ""
	m.session.LastFailure = ""
	return func() tea.Msg {
		out, err := repoTools.RunCommand(ctx, repoRoot, "git", "restore", ".")
		if err == nil && out.ExitCode == 0 {
			return infoMsg("已回滚工作区。")
		}
		out2, err2 := repoTools.RunCommand(ctx, repoRoot, "git", "checkout", "--", ".")
		if err2 == nil && out2.ExitCode == 0 {
			return infoMsg("已回滚工作区。")
		}
		return infoMsg("回滚失败。")
	}
}

func (m *tuiModel) appendLine(s string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	wasAtBottom := m.view.AtBottom()
	if s == "" {
		if m.log == "" {
			return
		}
		m.log = m.log + "\n"
		m.logLines = strings.Count(m.log, "\n") + 1
		m.view.SetContent(m.log)
		if wasAtBottom {
			m.view.GotoBottom()
		}
		return
	}
	if m.log == "" {
		m.log = s
		m.logLines = 1
		m.view.SetContent(m.log)
		if wasAtBottom {
			m.view.GotoBottom()
		}
		return
	}
	m.log = m.log + "\n" + s
	m.logLines = strings.Count(m.log, "\n") + 1
	m.view.SetContent(m.log)
	if wasAtBottom {
		m.view.GotoBottom()
	}
}

func (m *tuiModel) updateStatusLine() {
	repoPart := "repo: <unset>"
	if strings.TrimSpace(m.session.RepoRoot) != "" {
		repoPart = "repo: " + filepath.Base(m.session.RepoRoot)
	}
	pending := ""
	if strings.TrimSpace(m.session.PendingPatch) != "" {
		pending = " • pending patch"
	}
	m.statusLine = fmt.Sprintf("%s%s", repoPart, pending)
}

func (m tuiModel) renderHeader() string {
	return m.renderWelcomeBox()
}

func (m tuiModel) renderPromptArea() string {
	w := m.width
	if w <= 0 {
		w = 80
	}
	left := lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(m.statusLine)
	right := lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render("? for shortcuts")
	space := w - lipgloss.Width(left) - lipgloss.Width(right)
	if space < 1 {
		space = 1
	}
	hint := left + strings.Repeat(" ", space) + right
	divider := strings.Repeat("─", w)
	return strings.Join([]string{hint, divider, m.input.View()}, "\n")
}

func (m tuiModel) renderHelp() string {
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	text := strings.Join([]string{
		"Shortcuts:",
		"  ?        toggle this help",
		"  esc      close dialogs",
		"  ↑/↓      scroll",
		"  pgup/dn  scroll page",
		"  ctrl+c   quit",
		"",
		"自然语言控制示例：",
		"  使用 ./test-repo",
		"  看下 diff / 状态 / 回滚一下 / 继续重试",
		"  修复 xxx 让测试通过",
	}, "\n")
	return box.Render(text)
}

func (m tuiModel) renderConfirm() string {
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	text := strings.Join([]string{
		"Confirm",
		"  " + m.confirmPrompt,
		"  y / 应用    n / 取消    esc 关闭",
	}, "\n")
	return box.Render(text)
}

func (m tuiModel) renderWelcomeBox() string {
	cwd, _ := os.Getwd()
	modelProvider := ""
	modelName := ""
	if m.cfg != nil {
		modelProvider = strings.TrimSpace(m.cfg.Model.Provider)
		modelName = strings.TrimSpace(m.cfg.Model.Model)
	}
	if modelProvider == "" {
		modelProvider = "deepseek"
	}
	if modelName == "" {
		modelName = "deepseek-chat"
	}

	recent := "No recent activity"
	if m.state != nil {
		lines := make([]string, 0, 12)
		for _, r := range m.state.RecentRepos {
			if strings.TrimSpace(r) == "" {
				continue
			}
			lines = append(lines, "repo: "+r)
		}
		for _, g := range m.state.RecentGoals {
			if strings.TrimSpace(g) == "" {
				continue
			}
			lines = append(lines, "goal: "+g)
		}
		if len(lines) > 0 {
			recent = strings.Join(lines, "\n")
		}
	}

	title := fmt.Sprintf("eino-code %s", appVersion)
	left := strings.Join([]string{
		"Welcome back!",
		"",
		"Model: " + modelProvider + " · " + modelName,
		"Workdir: " + cwd,
	}, "\n")
	right := strings.Join([]string{
		"Tips for getting started",
		`- 说“使用 ./repo”切换仓库`,
		`- 说“修复 … 让测试通过”提出补丁`,
		`- 生成补丁后会询问是否应用`,
		"",
		"Recent activity",
		recent,
	}, "\n")

	panel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	w := m.width
	if w <= 0 {
		w = 90
	}
	leftW := 34
	if w < 60 {
		leftW = 24
	}
	rightW := w - leftW - 3
	if rightW < 20 {
		rightW = 20
	}
	leftCol := lipgloss.NewStyle().Width(leftW).Render(left)
	rightCol := lipgloss.NewStyle().Width(rightW).Render(right)
	content := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, " │ ", rightCol)
	return panel.Render(title + "\n" + content)
}

func (m *tuiModel) noteActivity() {
	if m.state == nil {
		m.state = &recentState{}
	}
	repo := strings.TrimSpace(m.session.RepoRoot)
	if repo != "" {
		m.state.NoteRepo(repo)
	}
	goal := strings.TrimSpace(m.session.LastGoal)
	if goal != "" {
		m.state.NoteGoal(goal)
	}
	saveRecentState(m.state)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m *tuiModel) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	headerHeight := lipgloss.Height(m.renderHeader())
	promptHeight := lipgloss.Height(m.renderPromptArea())
	extra := 0
	if m.showHelp {
		extra += lipgloss.Height(m.renderHelp())
	}
	if m.confirmMode && strings.TrimSpace(m.confirmPrompt) != "" {
		extra += lipgloss.Height(m.renderConfirm())
	}
	available := m.height - headerHeight - promptHeight - extra
	if available < 1 {
		available = 1
	}
	desired := m.logLines
	if desired < 1 {
		desired = 1
	}
	if desired > available {
		desired = available
	}
	m.view.Width = m.width
	m.view.Height = desired
}

func (m tuiModel) renderSessionStartLine() string {
	line := "⎿ SessionStart: ready"
	if !m.startedAt.IsZero() {
		line = line + " (" + m.startedAt.Format("15:04:05") + ")"
	}
	return line
}

func (m tuiModel) renderTryLine() string {
	return `❯ Try "使用 ./test-repo" 或 "修复 xxx 让测试通过"`
}
