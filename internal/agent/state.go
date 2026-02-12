package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type recentState struct {
	RecentRepos []string  `json:"recent_repos"`
	RecentGoals []string  `json:"recent_goals"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (s *recentState) NoteRepo(repo string) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return
	}
	s.RecentRepos = prependUnique(s.RecentRepos, repo, 5)
	s.UpdatedAt = time.Now()
}

func (s *recentState) NoteGoal(goal string) {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return
	}
	s.RecentGoals = prependUnique(s.RecentGoals, goal, 5)
	s.UpdatedAt = time.Now()
}

func prependUnique(list []string, v string, limit int) []string {
	out := make([]string, 0, minInt(limit, len(list)+1))
	out = append(out, v)
	for _, item := range list {
		if strings.TrimSpace(item) == "" || item == v {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func loadRecentState() *recentState {
	path := statePath()
	if path == "" {
		return &recentState{}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return &recentState{}
	}
	var st recentState
	if err := json.Unmarshal(b, &st); err != nil {
		return &recentState{}
	}
	return &st
}

func saveRecentState(st *recentState) {
	if st == nil {
		return
	}
	path := statePath()
	if path == "" {
		return
	}
	dir := filepath.Dir(path)
	_ = os.MkdirAll(dir, 0o755)
	b, err := json.Marshal(st)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}

func statePath() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, ".eino-cc", "state.json")
}
