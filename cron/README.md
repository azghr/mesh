# Cron Scheduler

Cron-style job scheduling for recurring tasks.

## Overview

This package provides a simple in-memory cron scheduler for recurring tasks. It supports standard 5-field cron expressions with overlap prevention.

## Features

- Standard cron expression parsing
- Overlap prevention (configurable per job)
- Manual job trigger
- Job status tracking
- Graceful shutdown

## Installation

```go
import "github.com/azghr/mesh/cron"
```

## Usage

### Basic Usage

```go
scheduler := cron.New()
scheduler.AddJob("daily-report", "0 0 * * *", func(ctx context.Context) error {
    return generateReport()
})
scheduler.Start()
defer scheduler.Stop()
```

### With Overlap Prevention

```go
scheduler.AddJob("cleanup", "*/5 * * * *", cleanupJob, cron.WithOverlap(false))
```

### Manual Trigger

```go
scheduler.Trigger(ctx, "daily-report")
```

### Job Status

```go
jobs := scheduler.ListJobs()
for _, job := range jobs {
    fmt.Printf("Job: %s, Next: %v, Running: %v\n", 
        job.Name, job.NextRun, job.Running)
}
```

## API

### `New() *Scheduler`

Creates a new scheduler.

### `scheduler.AddJob(name, schedule string, fn JobFunc, opts ...Option) error`

Adds a job. Returns error if job exists or schedule is invalid.

### `scheduler.Start()`

Starts the scheduler.

### `scheduler.Stop()`

Stops the scheduler gracefully.

### `scheduler.Trigger(ctx context.Context, name string) error`

Manually triggers a job.

### `scheduler.ListJobs() []JobStatus`

Lists all jobs with their status.

### `WithOverlap(allow bool) Option`

Controls whether job can run concurrently.

## Cron Expression Format

```
┌───────────── minute (0-59)
│ ┌───────────── hour (0-23)
│ │ ┌───────────── day of month (1-31)
│ │ │ ┌───────────── month (1-12)
│ │ │ │ ┌───────────── day of week (0-6, 0 = Sunday)
* * * * *
```

Special characters:
- `*` - any value
- `,` - value list (e.g., 1,3,5)
- `-` - range (e.g., 1-5)
- `/` - step (e.g., */5)

## Examples

Every minute:
```go
scheduler.AddJob("ping", "* * * * *", pingJob)
```

Every 5 minutes:
```go
scheduler.AddJob("cleanup", "*/5 * * * *", cleanupJob)
```

Every day at midnight:
```go
scheduler.AddJob("backup", "0 0 * * *", backupJob)
```

Every Monday at 9am:
```go
scheduler.AddJob("report", "0 9 * * 1", reportJob)
```

Every hour at 30 minutes:
```go
scheduler.AddJob("hourly", "30 * * * *", hourlyJob)
```