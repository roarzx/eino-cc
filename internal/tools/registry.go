package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error)
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

func (r *Registry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *Registry) List() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

type ToolResult struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

func SuccessResult(data interface{}) (json.RawMessage, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	result := ToolResult{
		OK:   true,
		Data: jsonData,
	}
	return json.Marshal(result)
}

func ErrorResult(err error) (json.RawMessage, error) {
	result := ToolResult{
		OK:    false,
		Error: err.Error(),
	}
	return json.Marshal(result)
}

func ParseArgs[T any](args json.RawMessage) (T, error) {
	var parsed T
	if err := json.Unmarshal(args, &parsed); err != nil {
		return parsed, fmt.Errorf("failed to parse args: %w", err)
	}
	return parsed, nil
}