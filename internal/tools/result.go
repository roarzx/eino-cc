package tools

import "encoding/json"

type Result struct {
	OK    bool        `json:"ok"`
	Data  any         `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
	Meta  *ResultMeta `json:"meta,omitempty"`
}

type ResultMeta struct {
	Truncated bool `json:"truncated,omitempty"`
}

func OK(data any) Result {
	return Result{OK: true, Data: data}
}

func Err(msg string) Result {
	return Result{OK: false, Error: msg}
}

func (r Result) JSON() string {
	b, _ := json.Marshal(r)
	return string(b)
}

