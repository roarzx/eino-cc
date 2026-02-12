## 目标（按你给的 Claude Code 界面做）
- **默认进入 TUI**：直接运行 `go run ./cmd/eino-code` 就进入全屏交互界面。
- **自然语言交互**：用户在底部输入自然语言；无需 `/diff` 也能说“看下 diff/回滚一下/继续重试/切换到 ./repo”。
- **Claude Code 风格布局**：顶部欢迎面板（Welcome back/版本/模型/当前目录/当前 repo/Recent activity）、中间输出区（工具 trace + agent 输出）、底部输入栏（❯），右下角提示 `? for shortcuts`。
- **确认点**：当出现 pending patch 时，界面弹出确认（y/N），或用自然语言“应用/取消”。

## 依赖引入（Bubble Tea 生态）
- 新增依赖：
  - `github.com/charmbracelet/bubbletea`
  - `github.com/charmbracelet/lipgloss`（样式）
  - `github.com/charmbracelet/bubbles/textarea` 或 `bubbles/textinput`（输入框）
  - 可选：`bubbles/viewport`（滚动输出区）、`bubbles/keymap`（快捷键提示）
- 约束：除 TUI 相关依赖外，不引