# Eino Claude Code MVP

A minimal viable coding agent based on Go + Eino that can search, read, generate patches, apply them, run tests, and automatically fix issues.

## Features

- **Repo structure reading**: Explore directory structure
- **Code search**: Ripgrep-based search for code patterns
- **File reading**: Read and analyze file contents
- **Patch generation**: Create unified diff patches
- **Patch application**: Apply patches using git apply
- **Test execution**: Run tests and capture results
- **Automatic fixing**: Up to 3 iterations for fixing failed tests
- **Observability**: Built-in logging and tracing
- **Tool-based architecture**: All operations through defined tools

## Architecture

```
User CLI Input
      ↓
Planner (generates JSON plan)
      ↓
Agent Loop (ReAct pattern)
  ├─ Search Code
  ├─ Read File  
  ├─ Generate Patch
  ├─ Apply Patch
  ├─ Run Tests
  └─ Reflect / Fix
      ↓
Output Final Diff + Summary
```

## Installation

```bash
go build -o eino-code ./cmd/eino-code
```

## Usage

```bash
./eino-code "fix login bug"
./eino-code "add error handling to the API"
./eino-code "update dependencies"
```

## Configuration

Edit `configs/default.yaml`:

```yaml
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

## Project Structure

```
eino-claude-code-mvp/
├── cmd/eino-code/main.go          # CLI entry point
├── internal/agent/                # Agent logic
│   ├── state.go                   # Run state management
│   ├── runner.go                  # Main runner
│   ├── react.go                   # ReAct agent
│   └── prompts.go                 # LLM prompts
├── internal/tools/                # Tool definitions
│   ├── registry.go                # Tool registry
│   ├── repo_tree.go               # Directory structure
│   ├── search_code.go             # Code search
│   ├── open_file.go               # File reading
│   ├── apply_patch.go             # Patch application
│   ├── git_diff.go                # Diff generation
│   └── run_cmd.go                 # Command execution
├── internal/observe/              # Observability
│   ├── logger.go                  # Structured logging
│   └── otel.go                    # Tracing (simplified)
├── internal/config/config.go      # Configuration
├── internal/repo/                 # Repository ops
└── configs/default.yaml           # Default config
```

## Tool System

The agent interacts with the repository through a defined set of tools:

| Tool | Description |
|------|-------------|
| `repo_tree` | Get directory structure |
| `search_code` | Search for code using ripgrep |
| `open_file` | Read file contents |
| `apply_patch` | Apply unified diff patch |
| `run_cmd` | Execute shell command |
| `git_diff` | Get git diff of changes |

## Development Status

✅ **Completed**:
- Project structure and configuration
- Tool system implementation
- Agent state management
- Basic ReAct loop
- CLI interface
- Logging and observability

🔄 **In Progress**:
- Eino LLM integration
- Advanced planning and reflection
- Real patch generation

## Next Steps

1. **Integrate Eino LLM** for planning and patch generation
2. **Add real patch generation** based on file analysis
3. **Implement proper error recovery** and reflection
4. **Add more sophisticated search** with embeddings
5. **Create test suite** for the agent itself

## License

MIT