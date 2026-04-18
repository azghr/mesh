package errors

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewAggregator(t *testing.T) {
	agg := NewAggregator()
	assert.NotNil(t, agg)
}

func TestRecordErrors(t *testing.T) {
	agg := NewAggregator(
		WithMinOccurrences(2),
	)

	err := DatabaseError("test", nil)
	agg.Record(err)
	agg.Record(err)

	grouped := agg.GetGroupedErrors()
	assert.Len(t, grouped, 1)
	assert.Equal(t, 2, grouped[0].Count)
}

func TestGetGroupedErrors(t *testing.T) {
	agg := NewAggregator(
		WithMinOccurrences(1),
	)

	agg.Record(DatabaseError("op1", nil))
	agg.Record(NotFoundError("resource", "id"))
	agg.Record(DatabaseError("op2", nil))

	grouped := agg.GetGroupedErrors()
	assert.GreaterOrEqual(t, len(grouped), 1)
}

func TestGetErrorCount(t *testing.T) {
	agg := NewAggregator()

	agg.Record(DatabaseError("test", nil))
	agg.Record(DatabaseError("test", nil))
	agg.Record(NotFoundError("resource", "id"))

	count := agg.GetErrorCount(ErrorTypeDatabase)
	assert.Equal(t, 2, count)
}

func TestGetErrorCountByCode(t *testing.T) {
	agg := NewAggregator()

	agg.Record(DatabaseError("SELECT", nil))
	agg.Record(DatabaseError("INSERT", nil))

	count := agg.GetErrorCountByCode("database:SELECT")
	assert.Equal(t, 1, count)
}

func TestGetTopErrors(t *testing.T) {
	agg := NewAggregator(
		WithMinOccurrences(1),
	)

	agg.Record(DatabaseError("test1", nil))
	agg.Record(DatabaseError("test2", nil))
	agg.Record(NotFoundError("res", "id"))

	top := agg.GetTopErrors(2)
	assert.Len(t, top, 2)
}

func TestStats(t *testing.T) {
	agg := NewAggregator()

	agg.Record(DatabaseError("test", nil))
	agg.Record(NotFoundError("res", "id"))

	stats := agg.Stats()
	assert.Equal(t, 2, stats["total_errors"])
}

func TestReset(t *testing.T) {
	agg := NewAggregator()

	agg.Record(DatabaseError("test", nil))
	agg.Reset()

	grouped := agg.GetGroupedErrors()
	assert.Len(t, grouped, 0)
}

func TestEnableAlert(t *testing.T) {
	agg := NewAggregator()

	var alertTriggered bool
	filter := GroupFilter{
		ErrType:   ErrorTypeDatabase,
		Code:      "test",
		Threshold: 3,
	}

	agg.EnableAlert(filter, func(group ErrorGroup) {
		alertTriggered = true
	})

	for i := 0; i < 3; i++ {
		agg.Record(DatabaseError("test", nil))
	}

	assert.True(t, alertTriggered)
}

func TestGroupErrorsByType(t *testing.T) {
	agg := NewAggregator()

	agg.Record(DatabaseError("test", nil))
	agg.Record(NotFoundError("res", "id"))
	agg.Record(DatabaseError("test2", nil))

	byType := agg.GroupErrorsByType()
	assert.Contains(t, byType, ErrorTypeDatabase)
}

func TestAggregatorWithWindow(t *testing.T) {
	agg := NewAggregator(
		WithWindow(1 * time.Second),
	)
	assert.NotNil(t, agg)
}

func TestAggregatorWithGroupBy(t *testing.T) {
	agg := NewAggregator(
		WithGroupBy("type", "message"),
	)
	assert.NotNil(t, agg)
}

func TestAggregatorString(t *testing.T) {
	agg := NewAggregator(
		WithMinOccurrences(1),
	)

	agg.Record(DatabaseError("test", nil))

	str := agg.String()
	assert.NotEmpty(t, str)
}
