package health

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChecker_Register(t *testing.T) {
	checker := NewChecker()

	checker.Register("test", func(ctx context.Context) error {
		return nil
	})

	assert.Equal(t, 1, checker.Count())
	assert.Contains(t, checker.List(), "test")
}

func TestChecker_Unregister(t *testing.T) {
	checker := NewChecker()

	checker.Register("test", func(ctx context.Context) error {
		return nil
	})

	require.Equal(t, 1, checker.Count())

	checker.Unregister("test")

	assert.Equal(t, 0, checker.Count())
	assert.NotContains(t, checker.List(), "test")
}

func TestChecker_Check_Success(t *testing.T) {
	checker := NewChecker()

	checker.Register("database", func(ctx context.Context) error {
		return nil
	})

	results := checker.Check(context.Background())

	require.Len(t, results, 1)
	assert.Equal(t, StatusPass, results["database"].Status)
	assert.Equal(t, "database", results["database"].Name)
	assert.NotEmpty(t, results["database"].Timestamp)
}

func TestChecker_Check_Failure(t *testing.T) {
	checker := NewChecker()

	checker.Register("database", func(ctx context.Context) error {
		return errors.New("connection failed")
	})

	results := checker.Check(context.Background())

	require.Len(t, results, 1)
	assert.Equal(t, StatusFail, results["database"].Status)
	assert.Equal(t, "connection failed", results["database"].Error)
}

func TestChecker_Check_Multiple(t *testing.T) {
	checker := NewChecker()

	checker.Register("database", func(ctx context.Context) error {
		return nil
	})

	checker.Register("redis", func(ctx context.Context) error {
		return nil
	})

	checker.Register("api", func(ctx context.Context) error {
		return errors.New("timeout")
	})

	results := checker.Check(context.Background())

	require.Len(t, results, 3)
	assert.Equal(t, StatusPass, results["database"].Status)
	assert.Equal(t, StatusPass, results["redis"].Status)
	assert.Equal(t, StatusFail, results["api"].Status)
}

func TestChecker_CheckOne_Success(t *testing.T) {
	checker := NewChecker()

	checker.Register("database", func(ctx context.Context) error {
		return nil
	})

	result, err := checker.CheckOne(context.Background(), "database")

	require.NoError(t, err)
	assert.Equal(t, StatusPass, result.Status)
	assert.Equal(t, "database", result.Name)
}

func TestChecker_CheckOne_NotFound(t *testing.T) {
	checker := NewChecker()

	_, err := checker.CheckOne(context.Background(), "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestChecker_Status_Pass(t *testing.T) {
	checker := NewChecker()

	checker.Register("database", func(ctx context.Context) error {
		return nil
	})

	status := checker.Status(context.Background())

	assert.Equal(t, StatusPass, status)
}

func TestChecker_Status_Fail(t *testing.T) {
	checker := NewChecker()

	checker.Register("database", func(ctx context.Context) error {
		return errors.New("connection failed")
	})

	status := checker.Status(context.Background())

	assert.Equal(t, StatusFail, status)
}

func TestChecker_Status_Mixed(t *testing.T) {
	checker := NewChecker()

	checker.Register("database", func(ctx context.Context) error {
		return nil
	})

	checker.Register("redis", func(ctx context.Context) error {
		return errors.New("connection failed")
	})

	status := checker.Status(context.Background())

	assert.Equal(t, StatusFail, status)
}

func TestChecker_Timeout(t *testing.T) {
	checker := NewChecker()

	slowCheck := func(ctx context.Context) error {
		select {
		case <-time.After(5 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	checker.RegisterWithOptions("slow", slowCheck, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	results := checker.Check(ctx)

	require.Len(t, results, 1)
	assert.Equal(t, StatusFail, results["slow"].Status)
	assert.Contains(t, results["slow"].Error, "context deadline exceeded")
}

func TestChecker_GetLastStatus(t *testing.T) {
	checker := NewChecker()

	checker.Register("database", func(ctx context.Context) error {
		return nil
	})

	// Run check to set last status
	checker.Check(context.Background())

	status, err := checker.GetLastStatus("database")

	require.NoError(t, err)
	assert.Equal(t, StatusPass, status)
}

func TestChecker_GetLastStatus_NotFound(t *testing.T) {
	checker := NewChecker()

	_, err := checker.GetLastStatus("nonexistent")

	assert.Error(t, err)
}

func TestChecker_GetLastChecked(t *testing.T) {
	checker := NewChecker()

	checker.Register("database", func(ctx context.Context) error {
		return nil
	})

	before := time.Now()
	checker.Check(context.Background())
	after := time.Now()

	lastChecked, err := checker.GetLastChecked("database")

	require.NoError(t, err)
	assert.True(t, lastChecked.After(before) || lastChecked.Equal(before))
	assert.True(t, lastChecked.Before(after) || lastChecked.Equal(after))
}

func TestChecker_List(t *testing.T) {
	checker := NewChecker()

	checker.Register("database", func(ctx context.Context) error {
		return nil
	})

	checker.Register("redis", func(ctx context.Context) error {
		return nil
	})

	list := checker.List()

	assert.Len(t, list, 2)
	assert.Contains(t, list, "database")
	assert.Contains(t, list, "redis")
}

func TestChecker_Count(t *testing.T) {
	checker := NewChecker()

	assert.Equal(t, 0, checker.Count())

	checker.Register("database", func(ctx context.Context) error {
		return nil
	})

	assert.Equal(t, 1, checker.Count())

	checker.Register("redis", func(ctx context.Context) error {
		return nil
	})

	assert.Equal(t, 2, checker.Count())
}

func TestResult_Duration(t *testing.T) {
	checker := NewChecker()

	checker.Register("fast", func(ctx context.Context) error {
		return nil
	})

	results := checker.Check(context.Background())

	assert.NotEmpty(t, results["fast"].Duration)
	assert.Contains(t, results["fast"].Duration, "s")
}
