package agent

import (
	"context"
	"testing"
)

func TestNewCriticLayer(t *testing.T) {
	c := NewCriticLayer()
	if c == nil {
		t.Fatal("constructor should return non-nil")
	}
	if !c.knownTables["dim_products"] {
		t.Error("should know dim_products")
	}
	if c.knownTables["non_existent_table"] {
		t.Error("should not know non_existent_table")
	}
}

func TestCriticValidate_CleanOutput(t *testing.T) {
	c := NewCriticLayer()
	output := &Output{
		Response:  "Sales in the East region dropped by 15% last week.",
		AgentName: "strategist",
	}

	result := c.Validate(context.Background(), output)
	if !result.IsValid {
		t.Errorf("clean output should be valid, got warnings: %v", result.Warnings)
	}
}

func TestCriticValidate_DetectsPII(t *testing.T) {
	c := NewCriticLayer()
	output := &Output{
		Response:  "Contact john@example.com or call 9876543210 for details.",
		AgentName: "liaison",
	}

	result := c.Validate(context.Background(), output)
	hasEmailWarning := false
	hasPhoneWarning := false
	for _, w := range result.Warnings {
		if containsStr(w, "email") {
			hasEmailWarning = true
		}
		if containsStr(w, "phone") {
			hasPhoneWarning = true
		}
	}

	if !hasEmailWarning {
		t.Error("should detect email PII")
	}
	if !hasPhoneWarning {
		t.Error("should detect phone PII")
	}
}

func TestCriticValidate_DetectsHallucinatedTables(t *testing.T) {
	c := NewCriticLayer()
	output := &Output{
		Response:  "I queried fact_orders and dim_customers for the results.",
		AgentName: "analyst",
	}

	result := c.Validate(context.Background(), output)
	hasHallucinationWarning := false
	for _, w := range result.Warnings {
		if containsStr(w, "hallucinated") {
			hasHallucinationWarning = true
			break
		}
	}

	if !hasHallucinationWarning {
		t.Error("should detect hallucinated tables: fact_orders, dim_customers")
	}
}

func TestCriticValidate_KnownTablesPass(t *testing.T) {
	c := NewCriticLayer()
	output := &Output{
		Response:  "Data from fact_sales and dim_products shows a positive trend.",
		AgentName: "analyst",
	}

	result := c.Validate(context.Background(), output)
	for _, w := range result.Warnings {
		if containsStr(w, "hallucinated") {
			t.Errorf("real tables should not trigger hallucination warning: %s", w)
		}
	}
}

func TestCriticValidate_EmptyResponse(t *testing.T) {
	c := NewCriticLayer()
	output := &Output{Response: "", AgentName: "analyst"}

	result := c.Validate(context.Background(), output)
	hasCoherenceWarning := false
	for _, w := range result.Warnings {
		if containsStr(w, "Empty response") {
			hasCoherenceWarning = true
		}
	}
	if !hasCoherenceWarning {
		t.Error("should warn about empty response")
	}
}

func TestCriticValidate_ShortResponse(t *testing.T) {
	c := NewCriticLayer()
	output := &Output{Response: "Yes.", AgentName: "analyst"}

	result := c.Validate(context.Background(), output)
	hasShortWarning := false
	for _, w := range result.Warnings {
		if containsStr(w, "short") {
			hasShortWarning = true
		}
	}
	if !hasShortWarning {
		t.Error("should warn about short response")
	}
}

func TestCriticValidate_InvalidConfidence(t *testing.T) {
	c := NewCriticLayer()
	output := &Output{
		Response:  "Here are the recommended actions.",
		AgentName: "planner",
		Actions: []ActionSuggestion{
			{Title: "Test", Confidence: 1.5},
			{Title: "OK", Confidence: 0.8},
		},
	}

	result := c.Validate(context.Background(), output)
	hasConfWarning := false
	for _, w := range result.Warnings {
		if containsStr(w, "Invalid confidence") {
			hasConfWarning = true
		}
	}
	if !hasConfWarning {
		t.Error("should warn about invalid confidence > 1.0")
	}
}

func TestMaskPII(t *testing.T) {
	c := NewCriticLayer()

	t.Run("masks long digit sequences", func(t *testing.T) {
		input := "Call 9876543210 now"
		masked := c.maskPII(input)
		if containsStr(masked, "9876543210") {
			t.Error("should have masked the phone number")
		}
		if !containsStr(masked, "[REDACTED]") {
			t.Error("should contain [REDACTED] marker")
		}
	})

	t.Run("preserves short numbers", func(t *testing.T) {
		input := "Revenue was 12345"
		masked := c.maskPII(input)
		if containsStr(masked, "[REDACTED]") {
			t.Error("should not mask short numbers")
		}
	})
}

func TestCheckCoherence_FailurePatterns(t *testing.T) {
	c := NewCriticLayer()

	patterns := []string{
		"I cannot access the database",
		"As an AI, I don't have real data",
		"The result is undefined",
	}

	for _, p := range patterns {
		warnings := c.checkCoherence(p)
		if len(warnings) == 0 {
			t.Errorf("should detect failure pattern in: %q", p)
		}
	}
}

func TestCheckPII_Aadhaar(t *testing.T) {
	c := NewCriticLayer()
	warnings := c.checkPII("My aadhaar is 123456789012")
	hasAadhaar := false
	for _, w := range warnings {
		if containsStr(w, "Aadhaar") {
			hasAadhaar = true
		}
	}
	if !hasAadhaar {
		t.Error("should detect Aadhaar numbers (12 digits)")
	}
}

func TestMaskPII_EdgeCases(t *testing.T) {
	c := NewCriticLayer()

	t.Run("no digits", func(t *testing.T) {
		input := "Hello world"
		masked := c.maskPII(input)
		if masked != input {
			t.Errorf("expected %q, got %q", input, masked)
		}
	})

	t.Run("exact 10 digits at start", func(t *testing.T) {
		input := "1234567890 is my number"
		masked := c.maskPII(input)
		if !containsStr(masked, "[REDACTED]") {
			t.Error("should mask exactly 10 digits at start")
		}
	})

	t.Run("exact 10 digits at end", func(t *testing.T) {
		input := "my number is 1234567890"
		masked := c.maskPII(input)
		if !containsStr(masked, "[REDACTED]") {
			t.Error("should mask exactly 10 digits at end")
		}
	})

	t.Run("multiple digits segments", func(t *testing.T) {
		input := "call 1234567890 or 0987654321 now"
		masked := c.maskPII(input)
		// Should mask both (maskPII recursively calls itself)
		if containsStr(masked, "1234567890") || containsStr(masked, "0987654321") {
			t.Errorf("should mask all digit segments >= 10, got: %q", masked)
		}
	})
}
