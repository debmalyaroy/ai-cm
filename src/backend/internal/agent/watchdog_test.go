package agent

import (
	"testing"
)

func TestSeverityForPriceDrop(t *testing.T) {
	tests := []struct {
		pct  float64
		want string
	}{
		{-25, "critical"},
		{-21, "critical"},
		{-20, "critical"}, // boundary
		{-15, "warning"},  // boundary
		{-18, "warning"},
		{-10, "info"},
		{-5, "info"},
	}
	for _, tc := range tests {
		got := severityForPriceDrop(tc.pct)
		if got != tc.want {
			t.Errorf("severityForPriceDrop(%.1f) = %q, want %q", tc.pct, got, tc.want)
		}
	}
}

func TestNewWatchdogAgent(t *testing.T) {
	w := NewWatchdogAgent(nil)
	if w == nil {
		t.Fatal("constructor should return non-nil")
	}
	if w.Name() != "watchdog" {
		t.Errorf("name = %q, want 'watchdog'", w.Name())
	}
}

func TestAnomalyTypes(t *testing.T) {
	if AnomalyTypePriceDrop != "price_drop" {
		t.Error("wrong constant")
	}
	if AnomalyTypeStockout != "stockout_risk" {
		t.Error("wrong constant")
	}
	if AnomalyTypeSalesAnomaly != "sales_anomaly" {
		t.Error("wrong constant")
	}
	if AnomalyTypeHighInventory != "high_inventory" {
		t.Error("wrong constant")
	}
}
