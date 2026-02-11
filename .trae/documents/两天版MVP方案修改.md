## 目标
- 将现有开发方案收敛为“两天内可落地的最小可用版本”，保证：在真实仓库中完成一次闭环（搜→读→产出diff→应用→跑测试→输出diff+摘要）。

## 修改范围（只改这份文档）
- 统一将“两周/Week 1/Week 2”改为“两天/Day 1/Day 2”。
- 缩减 MVP 能力：保留闭环必需项；将 OTEL、Planner/Reflect、自动修复多轮等移至 V2。
- 明确成功标准为“固定仓库 + 固定命令 + 可验收输出”。

## 修改预览（将下列内容替换对应章节）

### 1.1 替换为「两天版 MVP 能力（48h 可落地）」
```md
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
- IDE 插件 / 向量数据库 / 多 Agent / PR 自动创建
```

### 1.2 成功标准收敛为可验收口径
```md
## 1.2 成功标准

在一个指定的真实代码仓库中（repo_root 可配置），Agent 通过一次 CLI 指令完成闭环：

- 能搜索到与目标相关的代码位置（ripgrep）
- 能打开并读取目标文件内容
- 能输出一份可应用的 unified diff
- 能应用补丁并运行测试命令（默认：go test ./...）
- 测试通过时输出：最终 git diff + 变更摘要

验收输入示例：
- eino-code "修改某函数逻辑并保证 go test ./... 通过"

验收输出要求：
- 展示关键步骤日志（Search/Read/Patch/Apply/Test）
- 最终给出 git diff 与 3-6 行摘要
```

### 2.1/2.2 简化为线性流程（状态机移到 V2）
```md
## 2.1 系统流程（两天版 MVP）

User CLI 输入
      ↓
Agent Loop（单阶段）
  ├─ Search Code
  ├─ Read File
  ├─ Generate Patch
  ├─ Apply Patch
  ├─ Run Tests
  └─ Output Diff + Summary

> 说明：两天版不引入完整状态机与 Planner/Reflect 双阶段。

## 2.2 状态机（V2）

（保留现有 stateDiagram-v2，移动到“后续演进（V2）”或本节标注 V2）
```

### 6.1 双阶段架构改为「单阶段（两天版）」
```md
## 6.1 单阶段架构（两天版 MVP）

两天版采用单阶段 Agent：模型根据当前目标与工具返回结果，选择下一步工具并生成补丁。

- 不要求先生成严格 JSON 计划
- 默认只允许 1 次补丁尝试 + 可选 1 次重试（最多 2 轮）
```

### 7 Prompt 体系收敛
```md
# 7. Prompt 体系（两天版 MVP）

- System Prompt：约束模型（不编造文件、先 search 再 open、修改必须用 unified diff、diff 最小化）
- Patch Prompt：输入文件片段 + 目标，输出 diff --git 格式

（Planner Prompt / Reflect Prompt 移到 V2）
```

### 8 Loop 伪代码改为最多 2 轮（无 Reflect 依赖）
```md
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

    // 两天版：允许最多 1 次重试（下一轮继续 Search/Read/Patch）
}
```

### 9 配置文件降到最小可用
```md
repo_root: "."
model:
  provider: vision-proxy
  model: claude-4-sonnet

agent:
  max_iterations: 2

commands:
  test: "go test ./..."
```

### 10 OTEL 整段移到 V2
- 将「# 10 可观测性（OTEL）」整体移动到「# 13 后续演进（V2）」下。

### 12 开发排期改为 Day 1 / Day 2
```md
# 12. 开发排期（两天版）

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
```

## 实施步骤（你确认后我会执行）
1. 直接在该 md 文件中按上述预览替换对应章节并调整章节顺序（将 V2 内容后移）。
2. 通读检查标题层级与引用一致性（确保不再出现“两周/Week”字样）。
3. 给出一份最终“可验收的两天版 MVP”文档版本。

如果你确认，我就开始按上述预览对文件做实际修改。