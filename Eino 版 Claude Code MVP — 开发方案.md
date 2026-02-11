# Eino 版 Claude Code MVP — 开发方案

> 目标：基于 **Go + Eino** 实现一个可运行的“类 Claude Code” Coding Agent
> 能在真实代码仓库中：搜索 → 阅读 → 生成 diff → 应用 patch → 运行测试 → 输出 diff + 摘要（可选 1 次重试）

# 1. 项目目标与范围

## 1.1 MVP能力（两天版，48h 可落地）

本项目实现一个最小可用的 Coding Agent（单机 CLI 版）：

| 能力                     | 是否在两天版 MVP |
| ------------------------ | ---------------- |
| Repo 结构读取            | ✅               |
| 代码搜索（ripgrep）      | ✅               |
| 文件读取                 | ✅               |
| 生成 unified diff        | ✅               |
| 应用 patch（git apply）  | ✅               |
| 自动运行测试（固定命令） | ✅               |
| 输出最终 diff + 变更摘要 | ✅               |

**不在两天版 MVP（V2）**

- OTEL 可观测
- Planner（生成计划 JSON）
- Reflect（失败诊断与自修复多轮）
- 自动修复（>1 次重试）
- IDE 插件
- 向量数据库
- 多 Agent 协作
- PR 自动创建

## 1.2 成功标准

在一个指定的真实代码仓库中（repo_root 可配置），Agent 通过一次 CLI 指令完成闭环：

- 能搜索到与目标相关的代码位置（ripgrep）
- 能打开并读取目标文件内容
- 能输出一份可应用的 unified diff
- 能应用补丁并运行测试命令（默认：go test ./...）
- 测试通过时输出：最终 git diff + 变更摘要

验收输入示例：

- `eino-code "修改某函数逻辑并保证 go test ./... 通过"`

验收输出要求：

- 展示关键步骤日志（Search/Read/Patch/Apply/Test）
- 最终给出 git diff 与 3-6 行摘要

# 2. 总体架构

## 2.1 系统流程

```
User CLI 输入
      ↓
Agent Loop（单阶段）
  ├─ Search Code
  ├─ Read File
  ├─ Generate Patch
  ├─ Apply Patch
  ├─ Run Tests
  └─ Output Diff + Summary
      ↓
输出最终 Diff + Summary
```

## 2.2 状态机（V2）

```
stateDiagram-v2
    [*] --> Plan
    Plan --> Search
    Search --> Read
    Read --> Patch
    Patch --> Apply
    Apply --> Test
    Test --> Done: tests pass
    Test --> Reflect: tests fail
    Reflect --> Search
```

# 3. 项目目录结构

```
eino-claude-code-mvp/
├── cmd/
│   └── eino-code/
│       └── main.go
│
├── internal/
│   ├── agent/
│   │   ├── runner.go
│   │   ├── react.go
│   │   ├── prompts.go
│   │   └── state.go
│   │
│   ├── tools/
│   │   ├── registry.go
│   │   ├── repo_tree.go
│   │   ├── search_code.go
│   │   ├── open_file.go
│   │   ├── apply_patch.go
│   │   ├── run_cmd.go
│   │   └── git_diff.go
│   │
│   ├── repo/
│   │   ├── fs.go
│   │   ├── git.go
│   │   └── index.go
│   │
│   └── config/
│       └── config.go
│
├── configs/default.yaml
└── README.md
```

# 4. 核心数据结构（Memory / RunState）

Agent 不依赖上下文窗口，而依赖外部 Memory。

```
type RunState struct {
    RunID         string
    RepoRoot      string
    UserGoal      string

    Iteration     int
    MaxIterations int

    RepoNotes     []string
    OpenedFiles   map[string]string
    ModifiedFiles []string
    Attempts      []Attempt

    LastPatch     string
    LastCmd       string
    LastCmdOut    string
    LastCmdErr    string

    TestsPassed   bool
    DoneReason    string
}

type Attempt struct {
    Iteration int
    Cmd       string
    Stdout    string
    Stderr    string
    Diagnosis string
}
```

# 5. Tool 体系设计

Agent **只能通过工具操作仓库**。

## 5.1 Tool 列表

| Tool        | 作用          |
| ----------- | ------------- |
| repo_tree   | 获取目录结构  |
| search_code | ripgrep 搜索  |
| open_file   | 读取文件      |
| apply_patch | git apply     |
| run_cmd     | 执行测试      |
| git_diff    | 获取最终 diff |

## 5.2 Tool 返回标准

统一 JSON：

```
{
  "ok": true,
  "data": {},
  "error": ""
}
```

# 6. Agent 编排设计

## 6.1 单阶段架构（两天版 MVP）

两天版采用单阶段 Agent：模型根据当前目标与工具返回结果，选择下一步工具并生成补丁。

- 不要求先生成严格 JSON 计划
- 默认只允许 1 次补丁尝试 + 可选 1 次重试（最多 2 轮）

# 7. Prompt 体系

两天版仅保留最小 Prompt 集合。

## 7.1 System Prompt

约束模型行为：

- 不允许编造文件
- 必须先 search 再 open
- 修改必须用 unified diff
- diff 必须最小化
- 最多 2 轮尝试

## 7.2 Patch Prompt

输入：文件片段 + 目标
输出：`diff --git` 格式

# 8. Agent Loop 实现（伪代码）

```
for i := 1; i <= MaxIterations; i++ {

    agent.DecideNextAction()

    if state.LastPatch != "" {
        tools.ApplyPatch()
    }

    result := tools.RunCmd("go test ./...")

    if result.ExitCode == 0 {
        state.TestsPassed = true
        break
    }

    state.Attempts = append(state.Attempts, result)
}
```

# 9. 配置文件

configs/default.yaml

```
repo_root: "."
model:
  provider: vision-proxy
  model: claude-4-sonnet

agent:
  max_iterations: 2

commands:
  test: "go test ./..."
```

# 10. CLI 输出设计

运行示例：

```
$ eino-code "fix login bug"

Steps:
1. search login handler
2. open auth/login.go
3. generate patch
4. apply patch
5. run tests

Iteration 1:
- Patch applied
- Tests failed

Iteration 2:
- Fix tests
- Tests passed

Result:
✔ Success
git diff:
...
```

# 11. 开发排期（两天版）

## Day 1

- CLI + config（最小）
- repo_tree / search_code / open_file
- apply_patch / run_cmd
- git_diff + summary 输出

## Day 2

- 接入 Eino Agent（单阶段）
- Prompt 约束 + patch 生成
- 最多 2 轮尝试（1 次重试）
- 端到端验收：在真实仓库跑通一次闭环

# 12. 后续演进（V2）

- gopls / tree-sitter 索引
- Embedding 检索
- 多 Agent（Planner / Reviewer / Tester）
- PR 自动生成
- Web UI
- OTEL 可观测
- Planner（严格 JSON 计划）
- Reflect（失败诊断与多轮自修复）
- 状态机驱动的可控编排

# 结语

本方案目标不是复制 Claude Code，
而是构建 **可控、可观测、可扩展的 Coding Agent 基座**。

MVP 完成后，即可接入内部 AI 中台与 Langfuse，逐步演进为企业级 Coding Agent。
