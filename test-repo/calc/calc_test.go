package calc

import "testing"

func TestAdd(t *testing.T) {
	got := Add(1, 2)
	if got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
}

