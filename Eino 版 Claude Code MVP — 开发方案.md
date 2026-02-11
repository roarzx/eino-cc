# Eino 版 Claude Code MVP — 开发方案

> 目标：基于 **Go + Eino** 实现一个可运行的“类 Claude Code” Coding Agent
> 能在真实代码仓库中：搜索 → 阅读 → 生成 diff → 应用 patch → 运行测试 → 自动修复（循环）

# 1. 项目目标与范围

## 1.1 MVP能力（两周内落地）

本项目实现一个最小可用的 Coding Agent：

| 能力                     | 是否在 MVP |
| ------------------------ | ---------- |
| Repo 结构读取            | ✅          |
| 代码搜索（ripgrep）      | ✅          |
| 文件读取                 | ✅          |
| 生成 unified diff        | ✅          |
| git apply 应用 patch     | ✅          |
| 自动运行测试             | ✅          |
| 测试失败自动修复（≤3轮） | ✅          |
| 输出最终 diff + 变更摘要 | ✅          |
| OTEL 可观测              | ✅          |

**不在 MVP**

- IDE 插件
- 向量数据库
- 多 Agent 协作
- PR 自动创建

## 1.2 成功标准

Agent 能完成以下场景：

- 修复简单 bug
- 修改函数逻辑
- 添加小功能
- 修改后 tests/build 通过

# 2. 总体架构

## 2.1 系统流程

```
User CLI 输入
      ↓
Planner（生成计划）
      ↓
Agent Loop
  ├─ Search Code
  ├─ Read File
  ├─ Generate Patch
  ├─ Apply Patch
  ├─ Run Tests
  └─ Reflect / Fix
      ↓
输出最终 Diff + Summary
```

## 2.2 状态机（核心）

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
│   ├── observe/
│   │   ├── logger.go
│   │   └── otel.go
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

    PlanJSON      string

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

## 6.1 双阶段架构

### Stage 1 — Planner

生成 JSON 执行计划：

```
{
  "steps": [
    {"tool":"search_code","args":{"query":"login"}},
    {"tool":"open_file","args":{"path":"auth/login.go"}},
    {"tool":"apply_patch"},
    {"tool":"run_cmd"}
  ],
  "success_criteria":["tests pass"]
}
```

### Stage 2 — ReAct Loop

最大循环：**3轮**

每轮执行：

1. 选择工具
2. 生成 patch
3. 应用 patch
4. 运行 tests
5. 判断是否成功
6. 若失败 → 进入 Reflect

# 7. Prompt 体系

## 7.1 Planner Prompt

生成严格 JSON 计划。

## 7.2 ReAct System Prompt

约束模型行为：

- 不允许编造文件
- 必须先 search 再 open
- 修改必须用 unified diff
- diff 必须最小化
- 最多3轮修复

## 7.3 Patch Prompt

输入：文件片段 + 目标
输出：`diff --git` 格式

## 7.4 Reflect Prompt

输入：stderr
输出：

- 失败原因
- 下一步行动

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
    agent.ReflectAndContinue()
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
  max_iterations: 3

commands:
  test: "go test ./..."
  lint: "golangci-lint run ./..."
```

# 10. 可观测性（OTEL）

必须埋点：

| Span             | 说明     |
| ---------------- | -------- |
| agent.plan       | 生成计划 |
| agent.iteration  | 每轮循环 |
| tool.search_code | 搜索     |
| tool.open_file   | 读文件   |
| tool.apply_patch | 应用补丁 |
| tool.run_cmd     | 执行测试 |

# 11. CLI 输出设计

运行示例：

```
$ eino-code "fix login bug"

Plan:
1. search login handler
2. modify auth/login.go
3. run tests

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

# 12. 开发排期

## Week 1

- CLI + config
- repo_tree / search_code / open_file
- apply_patch / run_cmd
- OTEL 基础

## Week 2

- Eino Agent 接入
- Planner + Reflect
- 自动修复 loop
- 最终 diff 输出

# 13. 后续演进（V2）

- gopls / tree-sitter 索引
- Embedding 检索
- 多 Agent（Planner / Reviewer / Tester）
- PR 自动生成
- Web UI

# 结语

本方案目标不是复制 Claude Code，
而是构建 **可控、可观测、可扩展的 Coding Agent 基座**。

MVP 完成后，即可接入内部 AI 中台与 Langfuse，逐步演进为企业级 Coding Agent。