// Package cron provides cron-style job scheduling.
//
// This package implements a simple in-memory cron scheduler for recurring tasks.
// It supports standard 5-field cron expressions with overlap prevention.
//
// # Features
//
//   - Standard cron expression parsing (minute, hour, day, month, weekday)
//   - Overlap prevention (optional, configurable per job)
//   - Manual job trigger support
//   - Job status tracking
//   - Graceful shutdown
//
// # Usage
//
//	scheduler := cron.New()
//	scheduler.AddJob("daily-report", "0 0 * * *", handler)
//	scheduler.Start()
//
// For preventing overlapping executions:
//
//	scheduler.AddJob("cleanup", "*/5 * * * *", handler, cron.WithOverlap(false))
package cron

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// Cron expression has 5 fields
	cronFields = 5
)

// JobFunc is the function type for cron jobs
type JobFunc func(ctx context.Context) error

// Scheduler manages scheduled jobs
type Scheduler struct {
	mu      sync.RWMutex
	jobs    map[string]*Job
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// Job represents a scheduled job
type Job struct {
	name         string
	schedule     string
	fn           JobFunc
	nextRun      time.Time
	prevRun      time.Time
	running      bool
	runCount     int64
	allowOverlap bool
	lastError    error
}

// Config configures a job
type Config struct {
	Name         string
	Schedule     string
	Fn           JobFunc
	AllowOverlap bool
}

// New creates a new cron scheduler
func New() *Scheduler {
	return &Scheduler{
		jobs:   make(map[string]*Job),
		stopCh: make(chan struct{}),
	}
}

// AddJob adds a new job to the scheduler
//
// The schedule follows standard cron format:
//   - Minute: 0-59
//   - Hour: 0-23
//   - Day of month: 1-31
//   - Month: 1-12
//   - Day of week: 0-6 (0 = Sunday)
//
// Special characters:
//   - * matches any value
//   - , lists (e.g., 1,3,5)
//   - - ranges (e.g., 1-5)
//   - / steps (e.g., */5)
//
// Example:
//
//	// Every day at midnight
//	scheduler.AddJob("backup", "0 0 * * *", backupJob)
//
//	// Every 5 minutes
//	scheduler.AddJob("cleanup", "*/5 * * * *", cleanupJob)
//
//	// Mondays at 9am
//	scheduler.AddJob("report", "0 9 * * 1", reportJob)
func (s *Scheduler) AddJob(name, schedule string, fn JobFunc, opts ...Option) error {
	if fn == nil {
		return ErrJobFuncRequired
	}
	if err := validateSchedule(schedule); err != nil {
		return err
	}

	job := &Job{
		name:         name,
		schedule:     schedule,
		fn:           fn,
		allowOverlap: true,
	}

	for _, opt := range opts {
		opt(job)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.jobs[name]; ok {
		return ErrJobExists
	}

	s.jobs[name] = job
	return nil
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go s.run()
}

// Stop stops the scheduler gracefully
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()

	s.wg.Wait()
}

// Wait waits for all running jobs to complete
func (s *Scheduler) Wait() {
	s.wg.Wait()
}

// Trigger manually triggers a job by name
func (s *Scheduler) Trigger(ctx context.Context, name string) error {
	s.mu.RLock()
	job, ok := s.jobs[name]
	s.mu.RUnlock()

	if !ok {
		return ErrJobNotFound
	}

	s.runJob(ctx, job)
	return nil
}

// ListJobs returns all registered jobs
func (s *Scheduler) ListJobs() []JobStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statuses := make([]JobStatus, 0, len(s.jobs))
	for _, job := range s.jobs {
		statuses = append(statuses, JobStatus{
			Name:         job.name,
			Schedule:     job.schedule,
			NextRun:      job.nextRun,
			PrevRun:      job.prevRun,
			Running:      job.running,
			RunCount:     job.runCount,
			AllowOverlap: job.allowOverlap,
			LastError:    job.lastError,
		})
	}
	return statuses
}

// JobStatus represents the status of a job
type JobStatus struct {
	Name         string    `json:"name"`
	Schedule     string    `json:"schedule"`
	NextRun      time.Time `json:"next_run,omitempty"`
	PrevRun      time.Time `json:"prev_run,omitempty"`
	Running      bool      `json:"running"`
	RunCount     int64     `json:"run_count"`
	AllowOverlap bool      `json:"allow_overlap"`
	LastError    error     `json:"last_error,omitempty"`
}

// run starts the scheduler loop
func (s *Scheduler) run() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkJobs()
		}
	}
}

// checkJobs runs pending jobs
func (s *Scheduler) checkJobs() {
	now := time.Now()

	s.mu.RLock()
	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	s.mu.RUnlock()

	for _, job := range jobs {
		if job.nextRun.IsZero() {
			job.nextRun = s.nextTime(job.schedule, now)
			continue
		}

		if now.After(job.nextRun) || now.Equal(job.nextRun) {
			s.runJob(context.Background(), job)
			job.nextRun = s.nextTime(job.schedule, now)
		}
	}
}

// runJob executes a job
func (s *Scheduler) runJob(ctx context.Context, job *Job) {
	if !job.allowOverlap && job.running {
		return
	}

	s.wg.Add(1)
	defer s.wg.Done()

	s.mu.Lock()
	job.running = true
	s.mu.Unlock()

	job.lastError = job.fn(ctx)

	s.mu.Lock()
	job.running = false
	job.prevRun = time.Now()
	job.runCount++
	s.mu.Unlock()
}

// nextTime calculates the next run time for a cron expression
func (s *Scheduler) nextTime(schedule string, from time.Time) time.Time {
	parts, err := parseSchedule(schedule)
	if err != nil {
		return time.Time{}
	}

	// Simple implementation: find next matching time
	for i := 0; i < 60*60*24*366; i++ {
		t := from.Add(time.Duration(i) * time.Second)
		if matches(parts, t) {
			return t
		}
	}
	return time.Time{}
}

// parseSchedule parses a cron expression into parts
func parseSchedule(schedule string) ([][]int, error) {
	fields := make([][]int, cronFields)
	parts := strings.Fields(schedule)
	if len(parts) != cronFields {
		return nil, ErrInvalidSchedule
	}

	for i, part := range parts {
		fields[i] = parseField(part, fieldRanges[i])
	}
	return fields, nil
}

// parseField parses a single cron field
func parseField(expr string, valid []int) []int {
	expr = strings.TrimSpace(expr)
	if expr == "*" {
		return []int{-1}
	}

	var result []int
	for _, part := range strings.Split(expr, ",") {
		if strings.Contains(part, "/") {
			step := strings.Split(part, "/")
			if len(step) != 2 {
				continue
			}
			rangeParts := strings.Split(step[0], "-")
			start, end := valid[0], valid[len(valid)-1]
			if len(rangeParts) == 2 {
				start = atoi(rangeParts[0])
				end = atoi(rangeParts[1])
			} else if step[0] != "*" {
				start = atoi(step[0])
			}
			stepVal := atoi(step[1])
			if stepVal > 0 {
				for i := start; i <= end; i += stepVal {
					result = append(result, i)
				}
			}
		} else if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) == 2 {
				start := atoi(rangeParts[0])
				end := atoi(rangeParts[1])
				for i := start; i <= end; i++ {
					result = append(result, i)
				}
			}
		} else {
			result = append(result, atoi(part))
		}
	}
	return result
}

// atoi converts string to int
func atoi(s string) int {
	n := 0
	fmt.Sscanf(s, "%d", &n)
	return n
}

// fieldRanges defines valid ranges for each cron field
var fieldRanges = [][]int{
	{0, 59}, // minute
	{0, 23}, // hour
	{1, 31}, // day
	{1, 12}, // month
	{0, 6},  // weekday
}

// matches checks if time matches the cron expression
func matches(parts [][]int, t time.Time) bool {
	if !inRange(parts[0], t.Minute()) {
		return false
	}
	if !inRange(parts[1], t.Hour()) {
		return false
	}
	if !inRange(parts[2], t.Day()) {
		return false
	}
	if !inRange(parts[3], int(t.Month())) {
		return false
	}
	if !inRange(parts[4], int(t.Weekday())) {
		return false
	}
	return true
}

// inRange checks if value is in the set
func inRange(set []int, value int) bool {
	for _, v := range set {
		if v == value || v == -1 {
			return true
		}
	}
	return false
}

// Option configures a job
type Option func(*Job)

// WithOverlap controls whether the job can run concurrently
func WithOverlap(allow bool) Option {
	return func(j *Job) {
		j.allowOverlap = allow
	}
}

// Errors
var (
	ErrJobExists       = errors.New("job already exists")
	ErrJobNotFound     = errors.New("job not found")
	ErrJobFuncRequired = errors.New("job function is required")
	ErrInvalidSchedule = errors.New("invalid cron schedule")
)

// validateSchedule validates a cron expression
func validateSchedule(schedule string) error {
	parts := strings.Fields(schedule)
	if len(parts) != cronFields {
		return ErrInvalidSchedule
	}
	for i, part := range parts {
		if len(parseField(part, fieldRanges[i])) == 0 {
			return ErrInvalidSchedule
		}
	}
	return nil
}
