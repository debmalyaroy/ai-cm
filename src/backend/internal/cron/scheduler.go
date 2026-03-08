package cron

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Job defines an interface for tasks that can be scheduled.
type Job interface {
	// ID returns a unique identifier for the job used for DB locking.
	ID() string
	// ExpectedNextRun calculates the next run time after the given time.
	ExpectedNextRun(after time.Time) time.Time
	// Run executes the job logic.
	Run(ctx context.Context) error
}

// Scheduler manages distributed cron jobs using the database for locking.
type Scheduler struct {
	db       *pgxpool.Pool
	jobs     []Job
	nodeID   string
	cancel   context.CancelFunc
	interval time.Duration
}

// NewScheduler creates a new distributed cron scheduler.
func NewScheduler(db *pgxpool.Pool, nodeID string) *Scheduler {
	return &Scheduler{
		db:       db,
		nodeID:   nodeID,
		interval: 1 * time.Minute, // Tick every minute to check for jobs
	}
}

// Register adds a job to the scheduler.
func (s *Scheduler) Register(job Job) {
	s.jobs = append(s.jobs, job)
}

// Start begins the scheduling loop in a background goroutine.
func (s *Scheduler) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	go func() {
		slog.InfoContext(ctx, "Cron scheduler started", "node_id", s.nodeID, "jobs", len(s.jobs))
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		// Run once immediately on startup
		s.tick(ctx)

		for {
			select {
			case <-ctx.Done():
				slog.InfoContext(ctx, "Cron scheduler stopping")
				return
			case <-ticker.C:
				s.tick(ctx)
			}
		}
	}()
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// tick checks all registered jobs and attempts to acquire a lock to run them.
func (s *Scheduler) tick(ctx context.Context) {
	for _, job := range s.jobs {
		if err := s.tryRunJob(ctx, job); err != nil {
			slog.ErrorContext(ctx, "Job execution failed", "job_id", job.ID(), "error", err)
		}
	}
}

// tryRunJob attempts to lock the job in the DB and runs it if successful.
func (s *Scheduler) tryRunJob(ctx context.Context, job Job) error {
	now := time.Now().UTC()
	jobID := job.ID()

	// 1. Ensure job exists in DB
	_, err := s.db.Exec(ctx, `
		INSERT INTO cron_jobs (id, next_run) 
		VALUES ($1, $2) 
		ON CONFLICT (id) DO NOTHING`,
		jobID, job.ExpectedNextRun(now))
	if err != nil {
		return fmt.Errorf("failed to upsert job %s: %w", jobID, err)
	}

	// 2. Try to acquire the lock:
	// A job is runnable if next_run <= now AND (locked_by IS NULL OR locked_at < now - 30m, which is a stale lock)
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var lockedBy *string
	// Use FOR UPDATE SKIP LOCKED so multiple nodes don't block each other
	err = tx.QueryRow(ctx, `
		SELECT locked_by FROM cron_jobs 
		WHERE id = $1 
		  AND next_run <= $2 
		  AND (locked_by IS NULL OR locked_at < $2 - INTERVAL '30 minutes')
		FOR UPDATE SKIP LOCKED`,
		jobID, now).Scan(&lockedBy)

	if err != nil {
		// Either no rows (not time to run, or already locked by someone else actively) or error
		return nil // Not an error to simply skip
	}

	// 3. Acquire lock
	_, err = tx.Exec(ctx, `
		UPDATE cron_jobs 
		SET locked_by = $1, locked_at = $2, status = 'running', updated_at = $2 
		WHERE id = $3`,
		s.nodeID, now, jobID)
	if err != nil {
		return fmt.Errorf("failed to lock job %s: %w", jobID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	slog.InfoContext(ctx, "Job lock acquired, running", "job_id", jobID)

	// 4. Run the actual job
	runErr := job.Run(ctx)
	status := "idle"
	if runErr != nil {
		slog.ErrorContext(ctx, "Job encountered error during run", "job_id", jobID, "error", runErr)
		status = "error"
	}

	// 5. Release lock and calculate next run
	nextRun := job.ExpectedNextRun(now)
	_, err = s.db.Exec(context.Background(), `
		UPDATE cron_jobs 
		SET locked_by = NULL, locked_at = NULL, last_run = $1, next_run = $2, status = $3, updated_at = $1 
		WHERE id = $4`,
		now, nextRun, status, jobID)

	if err != nil {
		return fmt.Errorf("failed to release lock for job %s: %w", jobID, err)
	}

	slog.InfoContext(ctx, "Job completed, lock released", "job_id", jobID, "next_run", nextRun)
	return runErr
}

// --- Helpers for simple interval jobs ---

// IntervalJob is a simple job that runs on a fixed interval.
type IntervalJob struct {
	id       string
	interval time.Duration
	runFn    func(ctx context.Context) error
}

func NewIntervalJob(id string, interval time.Duration, runFn func(ctx context.Context) error) *IntervalJob {
	return &IntervalJob{id: id, interval: interval, runFn: runFn}
}

func (j *IntervalJob) ID() string {
	return j.id
}

func (j *IntervalJob) ExpectedNextRun(after time.Time) time.Time {
	return after.Add(j.interval)
}

func (j *IntervalJob) Run(ctx context.Context) error {
	return j.runFn(ctx)
}

// DailyJob is a simple job that runs once a day at a specific local hour and minute.
type DailyJob struct {
	id     string
	hour   int
	minute int
	runFn  func(ctx context.Context) error
}

func NewDailyJob(id string, hour, minute int, runFn func(ctx context.Context) error) *DailyJob {
	return &DailyJob{id: id, hour: hour, minute: minute, runFn: runFn}
}

func (j *DailyJob) ID() string {
	return j.id
}

func (j *DailyJob) ExpectedNextRun(after time.Time) time.Time {
	// Calculate the next occurrence of the specific hour/minute
	next := time.Date(after.Year(), after.Month(), after.Day(), j.hour, j.minute, 0, 0, after.Location())
	if next.Before(after) || next.Equal(after) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

func (j *DailyJob) Run(ctx context.Context) error {
	return j.runFn(ctx)
}
