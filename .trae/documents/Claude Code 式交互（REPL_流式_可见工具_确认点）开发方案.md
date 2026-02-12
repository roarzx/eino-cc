## 你指出的问题：为什么不是 Claude Code 式“自然语言交互”？

* 上一版把交互做成“命令式 REPL（/diff /apply）”更像传统 CLI。

* Claude Code 的关键是：**用户始终用自然语言说话**，工具/状态/确认点由系统在后台编排；斜杠命令只是补充而不是主入口。

下面给出“自然语言优先”的开发方案：用户可以说“看下 diff”“回滚一下”“把这个补丁应用了”“继续重试”，系统都能理解并执行。

## 目标体验（自然语言优先）

* **启动**：`eino-code --interactive -repo-root <repo>` 进入对话。

* **用户输入**：默认都是自然语言，不要求任何 `/xxx`。

* **系统输出**：

  * 先简短复述意图（1 行）

  * 需要工具时展示“正在…”与工具结果摘要

  * 需要用户确认时，用自然语言提问：

    * “我准备应用以下 diff，是否应用？(y/N)”

    * “要现在运行测试吗？(Y/n)”

* **仍保留快捷命令**：`/help` `/exit` 作为兜底，不作为核心交互方式。

## 核心设计：对话编排器（Dialogue Orchestrator）

### 1) 将用户输入分两类：

* **控制意图（Control Intent）**：看 diff、回滚、应用补丁、重试、查看状态、退出。

* **任务意图（Task Intent）**：修改代码、修复测试、重构、解释代码等（交给 Agent+工具闭环）。

### 2) 意图识别策略（优先本地规则，必要时用模型）

* **规则优先（确定性）**：

  * 包含关键词：

    * diff/变更/改了什么 → show\_diff

    * 回滚/撤销/恢复 → rollback

    * 应用/提交补丁/就这么改 → apply\_pending\_patch

    * 再试/继续/重试 → retry

    * 状态/当前进度 → status

    * 退出/结束 → exit

* **模糊时用模型判别（低 token、小模型也行）**：当规则命中不明显时，用一次轻量 LLM 分类输出 JSON：`{"intent":"task|diff|rollback|apply|retry|status|exit","confidence":0-1}`。

  * 目的：让“看下改动”“撤销一下刚才的修改”这种说法也能被理解。

### 3) 交互确认点（Claude Code 的关键）

* **补丁确认**（默认开启）：

  * Agent 产出 patch → 存入 `pendingPatch` → 展示预览 → 询问是否应用。

  * 用户回复自然语言也能识别：

    * “应用/是/好/确认/继续” → apply

    * “不要/取消/先别” → discard pending

* **测试确认**（可选开关）：

  * 默认自动跑 `go test`；如果开启 confirm，则在运行前询问。

## 会话状态（Session）

* `history/messages`：多轮上下文

* `pendingPatch`：待确认补丁

* `lastFailure`：上次失败摘要

* `lastToolTrace`：最近一次工具调用摘要（用于用户问“刚才做了什么”）

* `worktreeDirty`：工作区是否 dirty（用于提示回滚）

## 代码落地（文件级改造）

* [main.go](file:///home/zx/code/eino-cc/cmd/eino-code/main.go)：

  * 新增 `--interactive`

  * 读 stdin 循环：每轮先走 `classifyIntent()`，再分派到控制动作或 Runner.Step。

* 新增 `internal/agent/dialogue.go`（或同目录其他文件）：

  * `classifyIntent(text) -> Intent`

  * `handleControlIntent(intent, session)`: diff/status/rollback/apply/retry/exit

  * 统一自然语言确认：`askYesNo(prompt)`（接受 y/n 与中文“是/否/取消/继续”）

* [runner.go](file:///home/zx/code/eino-cc/internal/agent/runner.go)：

  * 抽出 `Step(ctx, session, userInput)`

  * 让 Agent 只负责“提出 patch + 验证策略”，不直接强制 apply；由 dialogue 层决定是否 apply。

* [prompts.go](file:///home/zx/code/eino-cc/internal/agent/prompts.go)：

  * 明确要求：输出 patch 后停下，等待用户确认（避免自动 apply 造成不可控）。

## 工具调用展示（像 Claude Code 的“正在做什么”）

* 对工具调用输出做统一渲染：

  * `【search_code】query=...` → `命中 N 条，涉及 X 个文件`

  * `【open_file】path=...` → `读取 L 行`

  * `【apply_patch】` → 显示 phase=check/apply/fallback 与错误摘要

  * `【run_cmd:test】` → `exit_code` + fail 关键行

## 验收标准（自然语言版本）

* 用户输入：

  * “看下当前 diff” → 展示 git diff

  * “回滚一下” → 恢复干净工作区

  * “把补丁应用了” → 若有 pendingPatch 则 apply

  * “继续重试” → 以 lastFailure 为上下文触发下一轮 Step

  * “修复 calc.Add 让测试通过” → 产生 pendingPatch → 用户说“应用” → `go test ./...` 通过

## 逐步逼近迭代顺序

1. 交互循环 + 规则意图识别 + 自然语言确认（先不接模型分类）
2. 引入轻量 LLM 意图分类兜底（提升口语表达理解）
3. Runner.Step + Session 上下文完善（更像连续对话）
4. 工具 trace 体验优化（摘要/折叠/截断）

## 安全与边界

* 命令仍严格白名单；补丁仍只允许 repo\_root 内已有文件

* 默认不开“创建新文件”，避免自然语言误触发大改动

