package agent

import (
	"context"
	"fmt"
	"strings"
)

// CriticLayer implements the Reflection pattern from design §2.
// It validates agent output before returning to the user.
type CriticLayer struct {
	// knownTables is the whitelist of valid table names in the schema.
	knownTables map[string]bool
}

// NewCriticLayer creates a new critic with the known schema tables.
func NewCriticLayer() *CriticLayer {
	return &CriticLayer{
		knownTables: map[string]bool{
			"dim_products":           true,
			"dim_sellers":            true,
			"dim_locations":          true,
			"fact_sales":             true,
			"fact_inventory":         true,
			"fact_competitor_prices": true,
			"fact_forecasts":         true,
			"chat_sessions":          true,
			"chat_messages":          true,
			"action_log":             true,
			"agent_sessions":         true,
			"agent_messages":         true,
			"agent_actions":          true,
			"business_context":       true,
			"agent_memory":           true,
		},
	}
}

// ValidationResult holds the result of critic validation.
type ValidationResult struct {
	IsValid  bool     `json:"is_valid"`
	Warnings []string `json:"warnings,omitempty"`
	Cleaned  string   `json:"cleaned,omitempty"` // sanitized response
}

// Validate checks an agent's output for quality and safety issues.
func (c *CriticLayer) Validate(ctx context.Context, output *Output) *ValidationResult {
	result := &ValidationResult{IsValid: true, Cleaned: output.Response}

	// Check 1: PII detection
	if warnings := c.checkPII(output.Response); len(warnings) > 0 {
		result.Warnings = append(result.Warnings, warnings...)
		result.Cleaned = c.maskPII(result.Cleaned)
	}

	// Check 2: Hallucinated table names
	if warnings := c.checkHallucinatedTables(output.Response); len(warnings) > 0 {
		result.Warnings = append(result.Warnings, warnings...)
	}

	// Check 3: Response coherence
	if warnings := c.checkCoherence(output.Response); len(warnings) > 0 {
		result.Warnings = append(result.Warnings, warnings...)
	}

	// Check 4: Confidence sanity
	if output.Actions != nil {
		for _, a := range output.Actions {
			if a.Confidence < 0 || a.Confidence > 1 {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Invalid confidence %.2f for action '%s'", a.Confidence, a.Title))
			}
		}
	}

	return result
}

// checkPII detects potential PII patterns in the response.
func (c *CriticLayer) checkPII(text string) []string {
	var warnings []string

	// Check for email-like patterns
	if containsPIIPattern(text, "@") && (containsPIIPattern(text, ".com") || containsPIIPattern(text, ".in") || containsPIIPattern(text, ".org")) {
		warnings = append(warnings, "Response may contain email addresses")
	}

	// Check for digit sequences
	digitCount := 0
	maxRun := 0
	for _, ch := range text {
		if ch >= '0' && ch <= '9' {
			digitCount++
			if digitCount > maxRun {
				maxRun = digitCount
			}
		} else {
			digitCount = 0
		}
	}

	if maxRun >= 12 {
		// If it's 12+, it qualifies as both
		warnings = append(warnings, "Response may contain phone numbers")
		warnings = append(warnings, "Response may contain Aadhaar numbers")
	} else if maxRun >= 10 {
		// Just a phone number
		warnings = append(warnings, "Response may contain phone numbers")
	}

	return warnings
}

// maskPII replaces potential PII with redacted markers.
func (c *CriticLayer) maskPII(text string) string {
	// Simple masking — in production, use regex-based NER
	result := text

	// Mask consecutive digit sequences >= 10
	var masked strings.Builder
	digitRun := 0
	digitStart := 0
	for i, ch := range result {
		if ch >= '0' && ch <= '9' {
			if digitRun == 0 {
				digitStart = i
			}
			digitRun++
		} else {
			if digitRun >= 10 {
				masked.WriteString(result[:digitStart])
				masked.WriteString("[REDACTED]")
				result = result[i:]
				return masked.String() + c.maskPII(result)
			}
			digitRun = 0
		}
	}
	if digitRun >= 10 {
		masked.WriteString(result[:digitStart])
		masked.WriteString("[REDACTED]")
		return masked.String()
	}

	return result
}

// checkHallucinatedTables looks for table names in the response that don't exist in the schema.
func (c *CriticLayer) checkHallucinatedTables(text string) []string {
	var warnings []string
	lower := strings.ToLower(text)

	// Common hallucinated table patterns
	suspiciousPrefixes := []string{"dim_", "fact_", "table_", "tbl_"}
	words := strings.Fields(lower)

	for _, word := range words {
		// Clean punctuation
		cleaned := strings.Trim(word, ".,;:()[]\"'`")
		for _, prefix := range suspiciousPrefixes {
			if strings.HasPrefix(cleaned, prefix) && !c.knownTables[cleaned] {
				warnings = append(warnings,
					fmt.Sprintf("Possible hallucinated table: '%s' is not in the schema", cleaned))
			}
		}
	}

	return warnings
}

// checkCoherence validates basic response quality.
func (c *CriticLayer) checkCoherence(text string) []string {
	var warnings []string

	if len(text) == 0 {
		warnings = append(warnings, "Empty response from agent")
		return warnings
	}

	if len(text) < 10 {
		warnings = append(warnings, "Response is suspiciously short")
	}

	// Check for common LLM failure patterns
	failPatterns := []string{
		"I cannot", "I don't have access", "as an AI",
		"I'm sorry, but I", "undefined", "null",
	}
	lowerText := strings.ToLower(text)
	for _, pattern := range failPatterns {
		if strings.Contains(lowerText, strings.ToLower(pattern)) {
			warnings = append(warnings, fmt.Sprintf("Response contains failure pattern: '%s'", pattern))
		}
	}

	return warnings
}

func containsPIIPattern(text, pattern string) bool {
	return strings.Contains(text, pattern)
}
