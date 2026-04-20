// Package idgen provides distributed ID generation using the Snowflake algorithm.
//
// This package generates unique 64-bit IDs across distributed systems.
// The ID is composed of: timestamp (41 bits) + machine ID (10 bits) + sequence (12 bits)
//
// The Snowflake ID provides:
//   - ~69 years of usable timestamps
//   - 1024 unique machine IDs per data center
//   - 4096 IDs per millisecond per machine
//   - IDs are time-ordered and monotonically increasing
//
// # Usage
//
//	Create a generator with a machine ID:
//
//	gen := idgen.New(1)
//	id := gen.Next()
//
//	// For string output (useful for JSON APIs)
//	str := gen.NextString()
package idgen

import (
	"errors"
	"strconv"
	"sync"
	"time"
)

const (
	// machineIDBits is the number of bits for machine ID
	machineIDBits uint = 10
	// sequenceBits is the number of bits for sequence
	sequenceBits uint = 12

	// maxMachineID is the maximum machine ID (2^machineIDBits - 1)
	maxMachineID int64 = 1<<machineIDBits - 1
	// maxSequence is the maximum sequence number (2^sequenceBits - 1)
	maxSequence int64 = 1<<sequenceBits - 1
)

// Custom epoch: 2024-01-01 00:00:00 UTC
// This provides ~69 years of usable timestamps
var customEpoch = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

// ErrInvalidMachineID is returned when machine ID is invalid
var ErrInvalidMachineID = errors.New("invalid machine ID: must be between 0 and 1023")

// Generator generates unique IDs using the Snowflake algorithm
type Generator struct {
	mu        sync.Mutex
	machineID int64
	sequence  int64
	lastTime  int64
}

// Option configures the generator
type Option func(*Generator)

// WithEpoch sets the custom epoch for ID generation
// Default: 2024-01-01 00:00:00 UTC
//
// Example:
//
//	gen, _ := idgen.New(1, idgen.WithEpoch(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)))
func WithEpoch(epoch time.Time) Option {
	epochMs := epoch.UnixMilli()
	return func(g *Generator) {
		customEpoch = epochMs
	}
}

// New creates a new Snowflake ID generator
//
// The machineID must be between 0 and 1023 (inclusive).
// For multi-machine deployments, assign unique machine IDs to each instance.
//
// Example:
//
//	gen := idgen.New(1) // machine ID 1
//	gen, _ := idgen.New(1, idgen.WithEpoch(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)))
func New(machineID int64, opts ...Option) (*Generator, error) {
	if machineID < 0 || machineID > maxMachineID {
		return nil, ErrInvalidMachineID
	}
	g := &Generator{
		machineID: machineID,
		sequence:  0,
		lastTime:  0,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g, nil
}

// Next generates the next unique ID
//
// Returns a monotonically increasing 64-bit ID.
// Thread-safe and can be called concurrently.
func (g *Generator) Next() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now().UnixMilli()
	curTime := now - customEpoch

	// If time went backward, wait until time recovers
	if curTime < g.lastTime {
		curTime = g.lastTime
	}

	// Reset sequence if time advanced
	if curTime > g.lastTime {
		g.sequence = 0
		g.lastTime = curTime
	} else {
		// Increment sequence, panic if overflow
		g.sequence++
		if g.sequence > maxSequence {
			// Wait for next millisecond
			g.sequence = 0
			curTime++
			g.lastTime = curTime
		}
	}

	// Construct ID: timestamp << (machineIDBits + sequenceBits) | machineID << sequenceBits | sequence
	id := (curTime << (machineIDBits + sequenceBits)) |
		(g.machineID << sequenceBits) |
		g.sequence

	return id
}

// NextString generates the next unique ID as a string
//
// Useful for JSON APIs and databases that don't support 64-bit integers.
func (g *Generator) NextString() string {
	return strconv.FormatInt(g.Next(), 10)
}

// String returns the string representation of the next ID
//
// Satisfies the fmt.Stringer interface.
func (g *Generator) String() string {
	return strconv.FormatInt(g.Next(), 10)
}

// NextUint64 generates the next unique ID as uint64
func (g *Generator) NextUint64() uint64 {
	return uint64(g.Next())
}

// GetMachineID returns the machine ID
func (g *Generator) GetMachineID() int64 {
	return g.machineID
}

// GetLastTime returns the last timestamp used
func (g *Generator) GetLastTime() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lastTime + customEpoch
}

// Info returns breakdown of the last generated ID
func (g *Generator) Info(id int64) Info {
	timestamp := (id >> (machineIDBits + sequenceBits)) + customEpoch
	machineID := (id >> sequenceBits) & maxMachineID
	sequence := id & maxSequence
	return Info{
		ID:        id,
		Timestamp: timestamp,
		MachineID: machineID,
		Sequence:  sequence,
	}
}

// Info holds the breakdown of a Snowflake ID
type Info struct {
	ID        int64 `json:"id"`
	Timestamp int64 `json:"timestamp"`
	MachineID int64 `json:"machine_id"`
	Sequence  int64 `json:"sequence"`
}

// NewGenerator is a global instance for simple use cases
// Machine ID must be set via initialization
var globalGenerator *Generator

// InitializeGlobal sets up the global generator
// Call this once at application startup
func InitializeGlobal(machineID int64) error {
	gen, err := New(machineID)
	if err != nil {
		return err
	}
	globalGenerator = gen
	return nil
}

// NextID generates the next unique ID using the global generator
// Must call InitializeGlobal first
func NextID() int64 {
	if globalGenerator == nil {
		panic("idgen: global generator not initialized, call InitializeGlobal first")
	}
	return globalGenerator.Next()
}

// NextIDString generates the next unique ID as string
func NextIDString() string {
	return strconv.FormatInt(NextID(), 10)
}
