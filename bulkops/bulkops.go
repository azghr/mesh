// Package bulkops provides batch insert/update for database efficiency.
//
// This package helps optimize database operations by batching multiple records:
// - Configurable batch size
// - Automatic batching for large datasets
// - Context cancellation support
// - Helper to break items into batches
//
// Example:
//
//	err := bulkops.Insert(ctx, users, 100, func(batch []User) error {
//	    return db.InsertUsers(ctx, batch)
//	})
package bulkops

import (
	"context"
)

// InsertFn is the function type for bulk insert.
type InsertFn[T any] func([]T) error

// Insert performs bulk insert with automatic batching.
// Items are processed in batches of batchSize.
func Insert[T any](ctx context.Context, items []T, batchSize int, insertFn InsertFn[T]) error {
	if len(items) == 0 {
		return nil
	}

	if batchSize <= 0 {
		batchSize = 100
	}

	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}

		batch := items[i:end]
		if err := insertFn(batch); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}

// UpdateFn is the function type for bulk update.
type UpdateFn[T any] func([]T) error

// Update performs bulk update with automatic batching.
func Update[T any](ctx context.Context, items []T, batchSize int, updateFn UpdateFn[T]) error {
	if len(items) == 0 {
		return nil
	}

	if batchSize <= 0 {
		batchSize = 100
	}

	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}

		batch := items[i:end]
		if err := updateFn(batch); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}

// DeleteFn is the function type for bulk delete.
type DeleteFn func([]string) error

// Delete performs bulk delete with automatic batching by IDs.
func Delete(ctx context.Context, ids []string, batchSize int, deleteFn DeleteFn) error {
	if len(ids) == 0 {
		return nil
	}

	if batchSize <= 0 {
		batchSize = 100
	}

	for i := 0; i < len(ids); i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}

		batch := ids[i:end]
		if err := deleteFn(batch); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}

// Result holds the result of a bulk operation.
type Result struct {
	Total     int
	Processed int
	Failed    int
}

// NewResult creates a new bulk operation result.
func NewResult(total int) Result {
	return Result{Total: total}
}

// AddProcessed increments the processed count.
func (r *Result) AddProcessed(n int) {
	r.Processed += n
}

// AddFailed increments the failed count.
func (r *Result) AddFailed(n int) {
	r.Failed += n
}

// InBatches breaks items into batches.
// Useful when you need more control over the batching process.
func InBatches[T any](items []T, batchSize int) [][]T {
	if len(items) == 0 || batchSize <= 0 {
		return nil
	}

	var batches [][]T
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}
	return batches
}
