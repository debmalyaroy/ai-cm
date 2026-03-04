package agent

import (
	"testing"
)

func TestNewStrategistAgent(t *testing.T) {
	s := NewStrategistAgent(nil, nil, nil)
	if s == nil {
		t.Fatal("constructor should return non-nil")
	}
	if s.Name() != "strategist" {
		t.Errorf("name = %q, want 'strategist'", s.Name())
	}
}
