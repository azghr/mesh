package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStmtCache_DefaultConfig(t *testing.T) {
	cfg := DefaultStmtCacheConfig()

	assert.Equal(t, 100, cfg.MaxStatements)
	assert.Equal(t, time.Hour, cfg.TTL)
}

func TestStmtCache_StatsInitialized(t *testing.T) {
	// Test that zero values don't panic
	var stats StmtCacheStats

	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
	assert.Equal(t, int64(0), stats.Evictions)
}
