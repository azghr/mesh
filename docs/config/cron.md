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
import (
    "context"
    "log"
    
    "github.com/azghr/mesh/cron"
)

func main() {
    scheduler := cron.New()
    
    err := scheduler.AddJob("daily-report", "0 0 * * *", func(ctx context.Context) error {
        log.Println("Running daily report...")
        return generateReport()
    })
    if err != nil {
        log.Fatal(err)
    }
    
    scheduler.Start()
    defer scheduler.Stop()
    
    // Keep running...
    select {}
}
```

### With Overlap Prevention

Prevent a job from running if the previous execution is still running:

```go
scheduler.AddJob("cleanup", "*/5 * * * *", cleanupJob, cron.WithOverlap(false))
```

### Manual Trigger

Manually trigger a job:

```go
err := scheduler.Trigger(ctx, "daily-report")
```

### Job Status

Check job status:

```go
jobs := scheduler.ListJobs()
for _, job := range jobs {
    fmt.Printf("Job: %s\n", job.Name)
    fmt.Printf("  Schedule: %s\n", job.Schedule)
    fmt.Printf("  Next Run: %v\n", job.NextRun)
    fmt.Printf("  Prev Run: %v\n", job.PrevRun)
    fmt.Printf("  Running: %v\n", job.Running)
    fmt.Printf("  Run Count: %d\n", job.RunCount)
    fmt.Printf("  Allow Overlap: %v\n", job.AllowOverlap)
}
```

## Cron Expression Format

```
┌───────────── minute (0-59)
│ ┌───────────── hour (0-23)
│ │ ┌───────────── day of month (1-31)
│ │ │ ┌───────────── month (1-12)
│ │ │ │ ┌───────────── day of week (0-6, 0 = Sunday)
* * * * *
```

### Special Characters

| Character | Description | Example |
|-----------|------------|---------|
| `*` | Any value | `* * * * *` = every minute |
| `,` | Value list | `0,30 * * * *` = at minute 0 and 30 |
| `-` | Range | `0 9-17 * * *` = every hour from 9am to 5pm |
| `/` | Step | `*/5 * * * *` = every 5 minutes |

### Examples

| Expression | Description |
|------------|-------------|
| `* * * * *` | Every minute |
| `*/5 * * * *` | Every 5 minutes |
| `0 * * * *` | Every hour at minute 0 |
| `0 0 * * *` | Daily at midnight |
| `0 0 * * 0` | Weekly on Sunday at midnight |
| `0 0 1 * *` | Monthly on the 1st |
| `0 9 * * 1-5` | Weekdays at 9am |
| `30 4 1,15 * *` | 1st and 15th at 4:30am |

## API Reference

### `New() *Scheduler`

Creates a new cron scheduler.

### `AddJob(name, schedule string, fn JobFunc, opts ...Option) error`

Adds a job to the scheduler.

Returns error if:
- Job already exists
- Invalid cron schedule
- Job function is nil

### `Start()`

Starts the scheduler. Jobs will run according to their schedules.

### `Stop()`

Stops the scheduler gracefully. Waits for running jobs to complete.

### `Trigger(ctx context.Context, name string) error`

Manually triggers a job by name.

Returns error if job not found.

### `ListJobs() []JobStatus`

Returns all registered jobs with their status.

### `JobFunc func(ctx context.Context) error`

Function type for jobs.

### `WithOverlap(allow bool) Option`

Configures whether the job can run concurrently.

- `WithOverlap(true)` - Allow concurrent executions (default)
- `WithOverlap(false)` - Prevent concurrent executions

## Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/azghr/mesh/cron"
)

func main() {
    scheduler := cron.New()
    
    // Job that runs every second (for testing)
    scheduler.AddJob("tick", "* * * * *", func(ctx context.Context) error {
        fmt.Println("Tick!", time.Now().Second())
        return nil
    })
    
    // Job with overlap prevention
    scheduler.AddJob("slow", "*/10 * * * *", func(ctx context.Context) error {
        log.Println("Starting slow job...")
        time.Sleep(2 * time.Second)
        log.Println("Slow job done")
        return nil
    }, cron.WithOverlap(false))
    
    scheduler.Start()
    defer scheduler.Stop()
    
    // Print status every 5 seconds
    for i := 0; i < 3; i++ {
        time.Sleep(5 * time.Second)
        jobs := scheduler.ListJobs()
        for _, job := range jobs {
            fmt.Printf("%s: runs=%d running=%v\n", 
                job.Name, job.RunCount, job.Running)
        }
    }
}
```