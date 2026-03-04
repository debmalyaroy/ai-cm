package agent

import (
	"testing"
)

func TestParseActions(t *testing.T) {
	t.Run("parses single action", func(t *testing.T) {
		input := `ACTION:
Title: Match Price on Cradles
Description: Competitor dropped price by 15%. Match to prevent share loss.
Type: price_update
Confidence: 0.92
---`
		actions := parseActions(input)
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].Title != "Match Price on Cradles" {
			t.Errorf("title = %q, want 'Match Price on Cradles'", actions[0].Title)
		}
		if actions[0].ActionType != "price_update" {
			t.Errorf("type = %q, want 'price_update'", actions[0].ActionType)
		}
		if actions[0].Confidence < 0.9 || actions[0].Confidence > 0.95 {
			t.Errorf("confidence = %f, want ~0.92", actions[0].Confidence)
		}
	})

	t.Run("parses multiple actions", func(t *testing.T) {
		input := `ACTION:
Title: Price Match
Description: Reduce price
Type: price_update
Confidence: 0.85
---
ACTION:
Title: Restock
Description: Order more units
Type: inventory_adjustment
Confidence: 0.90
---`
		actions := parseActions(input)
		if len(actions) != 2 {
			t.Fatalf("expected 2 actions, got %d", len(actions))
		}
		if actions[0].Title != "Price Match" {
			t.Errorf("first title = %q", actions[0].Title)
		}
		if actions[1].Title != "Restock" {
			t.Errorf("second title = %q", actions[1].Title)
		}
	})

	t.Run("handles empty input", func(t *testing.T) {
		actions := parseActions("")
		if len(actions) != 0 {
			t.Fatalf("expected 0 actions, got %d", len(actions))
		}
	})

	t.Run("handles action without terminator", func(t *testing.T) {
		input := `ACTION:
Title: Run Promotion
Description: Clear excess stock
Type: promotion_create
Confidence: 0.75`
		actions := parseActions(input)
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
	})
}

func TestSplitLines(t *testing.T) {
	t.Run("splits LF lines", func(t *testing.T) {
		lines := splitLines("a\nb\nc")
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines, got %d", len(lines))
		}
	})

	t.Run("splits CRLF lines", func(t *testing.T) {
		lines := splitLines("a\r\nb\r\nc")
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines, got %d", len(lines))
		}
	})

	t.Run("handles empty string", func(t *testing.T) {
		lines := splitLines("")
		if len(lines) != 0 {
			t.Fatalf("expected 0 lines for empty string, got %d: %v", len(lines), lines)
		}
	})
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"  hello  ", "hello"},
		{"hello", "hello"},
		{"\thello\t", "hello"},
		{"", ""},
		{"   ", ""},
	}
	for _, tc := range tests {
		got := trimSpace(tc.in)
		if got != tc.want {
			t.Errorf("trimSpace(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTrimPrefix(t *testing.T) {
	t.Run("matches prefix", func(t *testing.T) {
		val, ok := trimPrefix("Title: Hello World", "Title:")
		if !ok || val != "Hello World" {
			t.Errorf("got (%q, %v), want ('Hello World', true)", val, ok)
		}
	})

	t.Run("no match", func(t *testing.T) {
		_, ok := trimPrefix("Description: Something", "Title:")
		if ok {
			t.Error("should not match")
		}
	})
}

func TestNewPlannerAgent(t *testing.T) {
	p := NewPlannerAgent(nil, nil)
	if p == nil {
		t.Fatal("constructor should return non-nil")
	}
	if p.Name() != "planner" {
		t.Errorf("name = %q, want 'planner'", p.Name())
	}
}
