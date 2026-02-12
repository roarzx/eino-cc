## 目标定义（“逐步逼近”是什么）
- **短期目标（可用）**：稳定完成单一目标闭环：定位文件 → 阅读实现 → 生成可应用补丁 → 跑 `go test ./...` 通过 → 输出 diff。
- **中期目标（好用）**：对常见失败模式（补丁格式、路径不匹配、测试失败）有确定性修复路径，减少“模型输出不可执行”的概率。
- **长期目标（接近 Claude Code）**：引入多阶段规划/反思、回滚与更完整工具链，但仍以“最小可信闭环”作为核心约束。

## 阶段 0：先把当前闭环“锁死”为稳定可重复
- **修复工具协议冲突**：当前 [tools.go](file:///home/zx/code/eino-cc/internal/agent/tools.go#L63-L80) 在 `open_file` 后硬性禁止 `repo_tree/search_code`（`apply_patch required after open_file`），与真实修复流程（通常需要多次打开/搜索）冲突。
  - 方案：把“强制顺序”从“硬拦截”改为“软约束 + 状态机”。允许 `search_code/open_file` 多次迭代，但在 `run_cmd(test)` 前必须 `apply_patch`（仍保留安全性）。
- **补丁输入规范化更确定**：在 `apply_patch` 前增加 `git apply --check`（或等价验证），把“corrupt patch at line X”的定位信息结构化返回，便于 Agent 自修复。
- **移除/降级“从 transcript 自动抽 patch”**：目前 runner 会在 Agent 未调用 `apply_patch` 时从输出里抽 diff 再尝试应用，这很容易引入转义/截断导致 corrupt patch。
  - 方案：把自动抽取作为最后兜底，并且仅接受严格匹配的 `diff --git` 块（完整文件块、包含必要 header/hunk）。

## 阶段 1：让补丁生成变成“可计算”而非“靠模型运气”
- **基于已读文件生成补丁**：利用 `open_file` 得到的 `lastOpenedContent`，在 Agent 侧要求它输出“原行/新行 + 精确上下文”，并在本地拼装出规范 unified diff（确保 hunk header、行数、换行等正确）。
- **增强路径一致性**：统一使用 repo 内相对路径（`a/.. b/..`），并在补丁生成前用 repo_tree 验证目标文件存在，减少 `patch does not match repo files`。

## 阶段 2：把“测试失败→修复”做成可控策略库
- **结构化解析 go test 输出**：
  - 提取失败包/测试名/期望值等（你现在已有 expected/got 的尝试，但需要更通用）。
  - 将失败信息压缩成模型更易用的摘要（例如：失败文件、失败断言、最近打开文件路径）。
- **最小变更策略**：优先修改实现文件、禁止改测试；变更失败时自动回退到“重新定位→重新阅读→生成补丁”。

## 阶段 3：工具链逐步扩展（仍保持安全边界）
- **新增可选命令但默认关闭**：在配置允许时再开放 `go fmt`、`go test -run`、`go test ./... -count=1` 等；默认仍只允许 `test`。
- **新增只读工具（可选）**：例如 `stat_file`/`read_go_mod`/`list_dir`（优先复用现有 repo_tree/open_file 能覆盖的能力，避免滥加）。

## 阶段 4：接近 Claude Code 的“多阶段智能”但保持工程可控
- **Planner/Executor 两段式**：先产出计划（要改哪些文件、为什么、如何验证），再执行（工具调用）。
- **反思/回滚**：当连续失败时，输出“失败原因分类 + 下一步策略选择”，必要时自动撤销工作区变更（基于 git）。
- **上下文管理**：对已打开文件、已搜索查询做缓存，减少重复 token 消耗。

## 里程碑验收标准（每阶段必须能跑通）
- 阶段 0：对 `test-repo` 的 `calc.Add` 问题，稳定 1 次内完成修复并 `go test ./...` 通过。
- 阶段 1：对“单文件多处修改”能稳定生成可应用补丁，不再出现 corrupt patch。
- 阶段 2：对“常见断言失败（expected/got）”能自动提出并应用最小修复。
- 阶段 3：能在保持安全的前提下提升修复成功率（fmt/定向 test）。
- 阶段 4：在复杂任务上明显减少无效工具调用与盲改。

## 我将优先落地的改动顺序（最小风险→最大收益）
- 先改 `tools.go` 的硬拦截为状态机软约束（保留 run_cmd 前必须 apply_patch）。
- 再改 `runner.go`：降低 transcript 抽 patch 的权重，强制工具调用闭环。
- 最后改 `apply_patch.go`：增加 `--check` 与更结构化错误回传，提升自修复能力。