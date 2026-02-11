package agent

import "time"

type RunState struct {
	RunID         string
	RepoRoot      string
	UserGoal      string

	Iteration     int
	MaxIterations int

	PlanJSON string

	RepoNotes     []string
	OpenedFiles   map[string]string
	ModifiedFiles []string
	Attempts      []Attempt

	LastPatch     string
	LastCmd       string
	LastCmdOut    string
	LastCmdErr    string

	TestsPassed bool
	DoneReason  string
}

type Attempt struct {
	Iteration int
	Cmd       string
	Stdout    string
	Stderr    string
	Diagnosis string
}

func NewRunState(repoRoot, goal string, maxIterations int) *RunState {
	return &RunState{
		RunID:         generateRunID(),
		RepoRoot:      repoRoot,
		UserGoal:      goal,
		MaxIterations: maxIterations,
		Iteration:     0,
		OpenedFiles:   make(map[string]string),
		Attempts:      []Attempt{},
	}
}

func generateRunID() string {
	// Simple implementation - in production use UUID
	return "run-" + time.Now().Format("20060102-150405")
}

func (s *RunState) AddRepoNote(note string) {
	s.RepoNotes = append(s.RepoNotes, note)
}

func (s *RunState) AddOpenedFile(path, content string) {
	s.OpenedFiles[path] = content
}

func (s *RunState) AddModifiedFile(path string) {
	s.ModifiedFiles = append(s.ModifiedFiles, path)
}

func (s *RunState) AddAttempt(cmd, stdout, stderr, diagnosis string) {
	s.Attempts = append(s.Attempts, Attempt{
		Iteration: s.Iteration,
		Cmd:       cmd,
		Stdout:    stdout,
		Stderr:    stderr,
		Diagnosis: diagnosis,
	})
}

func (s *RunState) NextIteration() bool {
	s.Iteration++
	return s.Iteration <= s.MaxIterations
}

func (s *RunState) IsDone() bool {
	return s.TestsPassed || s.Iteration >= s.MaxIterations
}