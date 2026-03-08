package cron

import (
	"context"
	"errors"
	"testing"
	"time"
)

// --- NewScheduler ---

func TestNewScheduler(t *testing.T) {
	s := NewScheduler(nil, "node-1")
	if s == nil {
		t.Fatal("NewScheduler returned nil")
	}
	if s.nodeID != "node-1" {
		t.Errorf("nodeID = %q, want %q", s.nodeID, "node-1")
	}
	if s.interval != time.Minute {
		t.Errorf("interval = %v, want 1m", s.interval)
	}
	if len(s.jobs) != 0 {
		t.Errorf("initial jobs count = %d, want 0", len(s.jobs))
	}
}

// --- Register ---

func TestScheduler_Register(t *testing.T) {
	s := NewScheduler(nil, "node-1")

	job1 := NewIntervalJob("job1", time.Hour, func(_ context.Context) error { return nil })
	job2 := NewIntervalJob("job2", time.Hour, func(_ context.Context) error { return nil })

	s.Register(job1)
	s.Register(job2)

	if len(s.jobs) != 2 {
		t.Errorf("jobs count = %d, want 2", len(s.jobs))
	}
	if s.jobs[0].ID() != "job1" {
		t.Errorf("first job ID = %q, want %q", s.jobs[0].ID(), "job1")
	}
	if s.jobs[1].ID() != "job2" {
		t.Errorf("second job ID = %q, want %q", s.jobs[1].ID(), "job2")
	}
}

// --- Start / Stop (no jobs) ---

// TestScheduler_StartStop verifies Start/Stop lifecycle with zero jobs registered
// so that tryRunJob (which needs a DB) is never called.
func TestScheduler_StartStop(t *testing.T) {
	s := NewScheduler(nil, "test-node")
	// Register no jobs — tick() will iterate an empty slice; no DB calls.

	ctx := context.Background()
	s.Start(ctx)

	// Give the goroutine a moment to initialise before stopping.
	time.Sleep(10 * time.Millisecond)
	s.Stop()

	// Stop is idempotent — calling again must not panic.
	s.Stop()
}

func TestScheduler_Stop_BeforeStart(t *testing.T) {
	s := NewScheduler(nil, "node")
	// cancel is nil before Start — Stop should be a no-op.
	s.Stop()
}

// --- IntervalJob ---

func TestIntervalJob_ID(t *testing.T) {
	j := NewIntervalJob("my-job", time.Hour, func(_ context.Context) error { return nil })
	if j.ID() != "my-job" {
		t.Errorf("ID() = %q, want %q", j.ID(), "my-job")
	}
}

func TestIntervalJob_ExpectedNextRun(t *testing.T) {
	interval := 5 * time.Minute
	j := NewIntervalJob("j", interval, func(_ context.Context) error { return nil })

	now := time.Now()
	next := j.ExpectedNextRun(now)
	want := now.Add(interval)

	diff := next.Sub(want)
	if diff < -time.Millisecond || diff > time.Millisecond {
		t.Errorf("ExpectedNextRun: got %v, want %v (diff %v)", next, want, diff)
	}
}

func TestIntervalJob_Run_Success(t *testing.T) {
	called := false
	j := NewIntervalJob("j", time.Hour, func(_ context.Context) error {
		called = true
		return nil
	})

	if err := j.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !called {
		t.Error("runFn was not called")
	}
}

func TestIntervalJob_Run_Error(t *testing.T) {
	want := errors.New("run failed")
	j := NewIntervalJob("j", time.Hour, func(_ context.Context) error {
		return want
	})

	got := j.Run(context.Background())
	if got != want {
		t.Errorf("Run() error = %v, want %v", got, want)
	}
}

// --- DailyJob ---

func TestDailyJob_ID(t *testing.T) {
	j := NewDailyJob("daily-job", 9, 30, func(_ context.Context) error { return nil })
	if j.ID() != "daily-job" {
		t.Errorf("ID() = %q, want %q", j.ID(), "daily-job")
	}
}

func TestDailyJob_ExpectedNextRun_Future(t *testing.T) {
	// Use hour=23, minute=59 — ensure the next run is later today if now is early morning.
	j := NewDailyJob("j", 23, 59, func(_ context.Context) error { return nil })

	// Construct a reference time well before 23:59 today (midnight).
	base := time.Date(2026, 3, 8, 0, 0, 0, 0, time.Local)
	next := j.ExpectedNextRun(base)

	wantSameDay := time.Date(2026, 3, 8, 23, 59, 0, 0, time.Local)
	if !next.Equal(wantSameDay) {
		t.Errorf("ExpectedNextRun (future today) = %v, want %v", next, wantSameDay)
	}
}

func TestDailyJob_ExpectedNextRun_Past(t *testing.T) {
	// Use hour=0, minute=0 — if "after" is already past midnight the next run is tomorrow.
	j := NewDailyJob("j", 0, 0, func(_ context.Context) error { return nil })

	// Reference time: exactly midnight — next occurrence must be the NEXT day.
	base := time.Date(2026, 3, 8, 0, 0, 0, 0, time.Local)
	next := j.ExpectedNextRun(base)

	wantNextDay := time.Date(2026, 3, 9, 0, 0, 0, 0, time.Local)
	if !next.Equal(wantNextDay) {
		t.Errorf("ExpectedNextRun (past/equal) = %v, want %v", next, wantNextDay)
	}
}

func TestDailyJob_ExpectedNextRun_AlreadyPassed(t *testing.T) {
	// hour=8, after is 10:00 — scheduled time already passed, so next is tomorrow.
	j := NewDailyJob("j", 8, 0, func(_ context.Context) error { return nil })

	base := time.Date(2026, 3, 8, 10, 0, 0, 0, time.Local)
	next := j.ExpectedNextRun(base)

	want := time.Date(2026, 3, 9, 8, 0, 0, 0, time.Local)
	if !next.Equal(want) {
		t.Errorf("ExpectedNextRun (already passed) = %v, want %v", next, want)
	}
}

func TestDailyJob_Run_Success(t *testing.T) {
	called := false
	j := NewDailyJob("j", 9, 0, func(_ context.Context) error {
		called = true
		return nil
	})

	if err := j.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !called {
		t.Error("runFn was not called")
	}
}

func TestDailyJob_Run_Error(t *testing.T) {
	want := errors.New("daily run failed")
	j := NewDailyJob("j", 9, 0, func(_ context.Context) error { return want })

	got := j.Run(context.Background())
	if got != want {
		t.Errorf("Run() error = %v, want %v", got, want)
	}
}

// --- Job interface conformance ---

func TestIntervalJob_ImplementsJob(t *testing.T) {
	var _ Job = NewIntervalJob("j", time.Hour, func(_ context.Context) error { return nil })
}

func TestDailyJob_ImplementsJob(t *testing.T) {
	var _ Job = NewDailyJob("j", 9, 0, func(_ context.Context) error { return nil })
}
