# eino-cc（两天版 MVP）

基于 Go + CloudWeGo Eino 的最小可用 Coding Agent（CLI），在真实代码仓库中完成一次闭环：
search → read → patch → apply → test → 输出 diff。

## 前置条件

- Go 1.21+
- 目标仓库必须是 Git 仓库（工具依赖 `git`)
- 本机安装 `rg`（ripgrep，用于代码搜索）
- DeepSeek API Key（通过环境变量提供，默认读取 `DEEPSEEK_KEY`）

## 快速开始

1) 在目标仓库所在机器设置环境变量（不要把 key 写进代码仓库）：

```bash
export DEEPSEEK_KEY="sk-xxxxxxxx"
```

2) 在本仓库目录运行（指定要操作的目标仓库根目录）：

```bash
go run ./cmd/eino-code -repo-root /path/to/your/repo "修改某函数逻辑并保证 go test ./... 通过"
```

## 编译生成产物

```bash
go build -o bin/eino-code ./cmd/eino-code
```

运行产物：

```bash
./bin/eino-code -repo-root /path/to/your/repo "修改某函数逻辑并保证 go test ./... 通过"
```

## 配置文件

默认配置在 [default.yaml](file:///home/zx/code/eino-cc/configs/default.yaml)：

- `repo_root`：目标仓库路径（默认 "."，推荐通过 `-repo-root` 覆盖）
- `model.provider`：默认 `deepseek`
- `model.model`：默认 `deepseek-chat`
- `model.base_url`：默认 `https://api.deepseek.com`（如走网关可改为内网地址）
- `model.api_key_env`：默认 `DEEPSEEK_KEY`
- `agent.max_iterations`：默认 2（最多 2 轮工具调用/推理）
- `commands.test`：默认 `go test ./...`（当前仅允许执行 `test`）

## CLI 参数

运行入口：[main.go](file:///home/zx/code/eino-cc/cmd/eino-code/main.go)

```bash
go run ./cmd/eino-code \
  -config configs/default.yaml \
  -repo-root /path/to/your/repo \
  -model-provider deepseek \
  -model deepseek-chat \
  -base-url https://api.deepseek.com \
  -api-key-env DEEPSEEK_KEY \
  -max-iterations 2 \
  -test-cmd "go test ./..." \
  "你的目标描述"
```

说明：
- `-repo-root`：推荐必填，否则默认使用当前目录作为目标仓库
- `-test-cmd`：用来覆盖默认测试命令（目前 agent 仅允许通过 `run_cmd(name="test")` 执行）

## 工具能力（MVP）

Agent 通过工具操作目标仓库（见 [internal/tools](file:///home/zx/code/eino-cc/internal/tools)）：

- `repo_tree`：列出 git tracked 文件（`git ls-files`）
- `search_code`：ripgrep 搜索（`rg --line-number --column`）
- `open_file`：读取文件片段（带行号输出）
- `apply_patch`：应用 unified diff（`git apply`）
- `run_cmd`：执行允许的命令（当前仅 `test`）
- `git_diff`：输出最终 diff（`git diff`）

## 常见问题

### 1) 运行时报 “missing api key env DEEPSEEK_KEY”

说明当前进程没读到环境变量。先执行：

```bash
export DEEPSEEK_KEY="sk-xxxxxxxx"
```

或在配置中修改 `model.api_key_env` 为你们实际的变量名，并确保在运行时已导出。

### 2) 运行时报找不到 `rg`

安装 ripgrep 后重试（例如 Linux 上可用包管理器安装）。

### 3) 目标仓库不是 git 仓库

当前 MVP 的 `repo_tree`/`git_diff`/`apply_patch` 依赖 git；请在 git 仓库根目录运行，或初始化 git。

## 测试用仓库

仓库内置一个可直接使用的测试目录：`test-repo`。

初始化并运行示例：

```bash
cd test-repo
git init
git add .
git commit -m "init"
cd ..
./bin/eino-code -repo-root ./test-repo "修复 calc.Add 使测试通过"
```

## 当前限制

- 仅支持 DeepSeek（通过 OpenAI 兼容接口调用）
- 仅允许执行 `test` 一个命令
- 没有做 OTEL/Planner/多 Agent 等（按“两天版 MVP”范围）
