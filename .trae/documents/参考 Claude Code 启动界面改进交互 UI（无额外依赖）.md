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
- 约束：除 TUI 相关依赖外，不引入额外框架；现有 Agent/Tools 逻辑复用。

## 架构设计（把 UI 和“对话编排”分离）
### 1) 现有逻辑层保留
- `dialogue.go` 继续负责：
  - session（RepoRoot/LastGoal/LastFailure/PendingPatch）
  - 自然语言意图识别（diff/status/rollback/apply/retry/exit）
  - repo 解析与切换（“使用 ./test-repo”）
- `runner.go` 保留 `Propose()`：产出 patch（不自动 apply），并接受 trace callback。

### 2) 新增 TUI 层（Bubble Tea model）
- 新增一个 TUI model（建议放 `internal/agent/tui.go` 或 `internal/tui/model.go`）：
  - 状态：Welcome/Chat/ConfirmPatch/Help
  - 数据：session、cfg、工作目录、最近活动、输出日志（ring buffer）、当前输入框
  - 事件：
    - 用户输入 msg（提交一行）
    - 工具 trace msg（repo_tree/search/open/run 等摘要）
    - agent 输出 msg（流式或段落式）
    - patch 提案 msg（进入 ConfirmPatch）
    - apply/test 结果 msg

### 3) 输出渲染（对齐 Claude Code 样式）
- Welcome panel：用 lipgloss 画框与两列布局（左图/右 tips）。
- 中间 viewport：显示按时间追加的日志块：
  - `⎿ SessionStart: ...`（启动检查/告警）
  - `【search_code】...` / `【open_file】...` / `【apply_patch】phase=...` 等 trace
  - agent 文本（计划、解释、diff 预览）
- 底部输入区：`❯` + textarea/textinput；右侧显示 `? for shortcuts`。

## 交互细节（自然语言 + 快捷键）
- **自然语言控制**：
  - “看下 diff”→ git_diff
  - “回滚一下”→ git restore
  - “继续重试”→ 以上次目标+失败摘要再 propose
  - “使用 ./repo”/“切换到 /path/to/repo”→ 切换 session.RepoRoot
- **快捷键（可选但建议）**：
  - `?` 打开 shortcuts
  - `ctrl+c` 退出
  - `enter` 提交
  - `esc` 关闭弹窗（help/confirm）
- **patch 确认弹窗**：
  - 显示 diff（可滚动）
  - `y` / `n` 选择；或输入“应用/取消”也能触发。

## Recent activity（第 2 步实现，增强产品感）
- 在 `~/.eino-cc/state.json` 记录：最近 N 个 repo、最近 N 个目标、上次时间。
- 启动时展示到 Welcome panel 的 Recent activity 区域。

## 代码改动清单（文件级）
- [cmd/eino-code/main.go](file:///home/zx/code/eino-cc/cmd/eino-code/main.go)：改为默认启动 TUI；保留参数（config/model/test-cmd/fmt-cmd），可选接受初始 goal。
- [internal/agent/dialogue.go](file:///home/zx/code/eino-cc/internal/agent/dialogue.go)：抽出“处理一条输入”的纯函数/方法，便于 TUI 调用；stdout 打印改为写入 UI 的事件流。
- [internal/agent/tools.go](file:///home/zx/code/eino-cc/internal/agent/tools.go)：trace callback 改为向 TUI 发送 msg（但保持 tool 本身 JSON 输出不变）。
- [internal/agent/runner.go](file:///home/zx/code/eino-cc/internal/agent/runner.go)：在 Propose 的 trace/输出处支持把内容送入 TUI（通过回调/chan）。
- 新增（必要）：TUI 入口文件（`tui.go`）与可选 state 存储文件（`state.go`）。
- `go.mod/go.sum`：加入 bubbletea/lipgloss/bubbles 依赖。

## 验收标准
- `go run ./cmd/eino-code` 直接进入 TUI（无 `--interactive`）。
- 在 TUI 中输入“使用 ./test-repo”能切换 repo，并在面板/状态栏体现。
- 输入“修复 calc.Add 让测试通过”→ 显示计划/trace→ 生成 patch→ 弹出确认→ 选择 y 后 apply+test→ 显示 diff。
- `?` 能打开快捷帮助。

## 迭代顺序（确保每步可跑通）
1) 引入 bubbletea + 最小 TUI（viewport + input），先把现有 dialogue 输出搬进 UI。
2) Welcome panel + shortcuts。
3) Confirm patch 弹窗 + apply/test 结果展示。
4) Recent activity 持久化。

你确认后，我会按以上顺序实现，并用 `test-repo` 做端到端演示。