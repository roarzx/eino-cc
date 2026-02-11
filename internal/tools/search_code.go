package tools

import (
	"context"
	"strconv"
	"strings"
)

type SearchCodeParams struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}

type SearchMatch struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Text   string `json:"text"`
}

type SearchCodeData struct {
	Query   string        `json:"query"`
	Matches []SearchMatch `json:"matches"`
}

func SearchCode(ctx context.Context, repoRoot string, params *SearchCodeParams) Result {
	if params == nil || strings.TrimSpace(params.Query) == "" {
		return Err("missing query")
	}
	maxResults := 200
	if params.MaxResults > 0 {
		maxResults = params.MaxResults
	}

	out, err := RunCommand(ctx, repoRoot, "rg", "--line-number", "--column", "--no-heading", "--smart-case", params.Query, ".")
	if err != nil {
		return Err("rg execution failed")
	}
	if out.ExitCode != 0 && strings.TrimSpace(out.Stdout) == "" {
		return OK(SearchCodeData{Query: params.Query, Matches: nil})
	}

	lines := splitLines(out.Stdout)
	matches := make([]SearchMatch, 0, minInt(len(lines), maxResults))
	truncated := false
	for _, line := range lines {
		path, ln, col, text, ok := parseRGLine(line)
		if !ok {
			continue
		}
		matches = append(matches, SearchMatch{Path: path, Line: ln, Column: col, Text: text})
		if len(matches) >= maxResults {
			truncated = true
			break
		}
	}

	return Result{OK: true, Data: SearchCodeData{Query: params.Query, Matches: matches}, Meta: &ResultMeta{Truncated: truncated}}
}

func parseRGLine(line string) (path string, ln int, col int, text string, ok bool) {
	parts := strings.SplitN(line, ":", 4)
	if len(parts) < 4 {
		return "", 0, 0, "", false
	}
	lnI, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, 0, "", false
	}
	colI, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", 0, 0, "", false
	}
	return parts[0], lnI, colI, parts[3], true
}

