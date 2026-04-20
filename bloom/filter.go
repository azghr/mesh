// Package bloom provides a Bloom filter implementation using Redis for storage.
//
// A Bloom filter is a space-efficient probabilistic data structure
// that can tell you if an element is definitely not in a set or possibly in a set.
//
// # Features
//
//   - Redis-backed storage for distributed use
//   - Configurable expected items and false positive rate
//   - Add, Exists, and Reset operations
//   - Multiple independent filters
//
// # Usage
//
//	bf := bloom.New(redisClient, "my-filter", 10000, 0.01)
//	bf.Add(ctx, "item")
//	exists := bf.Exists(ctx, "item")
package bloom

import (
	"context"
	"hash/fnv"
	"math"

	"github.com/redis/go-redis/v9"
)

const (
	defaultHashes = 7
)

// Filter represents a Bloom filter
type Filter struct {
	client        *redis.Client
	name          string
	expectedItems int
	falsePositive float64
	numBits       uint64
	numHashes     uint64
}

// New creates a new Bloom filter
//
//	expectedItems: number of items expected to be added
//	falsePositive: desired false positive rate (0.0 to 1.0)
//
// Example:
//
//	bf := bloom.New(client, "users", 100000, 0.01)
func New(client *redis.Client, name string, expectedItems int, falsePositive float64) *Filter {
	numBits := optimalNumBits(expectedItems, falsePositive)
	numHashes := optimalNumHashes(float64(expectedItems), float64(numBits))

	return &Filter{
		client:        client,
		name:          name,
		expectedItems: expectedItems,
		falsePositive: falsePositive,
		numBits:       numBits,
		numHashes:     numHashes,
	}
}

// Add adds an item to the filter
//
//	bf.Add(ctx, "user:123")
func (f *Filter) Add(ctx context.Context, item string) error {
	indices := f.getIndices(item)

	pipe := f.client.Pipeline()
	for _, idx := range indices {
		pipe.SetBit(ctx, f.name, int64(idx), 1)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// Exists checks if an item might be in the filter
//
// Returns true if the item possibly exists, false if definitely not
func (f *Filter) Exists(ctx context.Context, item string) (bool, error) {
	indices := f.getIndices(item)

	for _, idx := range indices {
		bit, err := f.client.GetBit(ctx, f.name, int64(idx)).Result()
		if err != nil {
			return false, err
		}
		if bit == 0 {
			return false, nil
		}
	}
	return true, nil
}

// AddMany adds multiple items to the filter
func (f *Filter) AddMany(ctx context.Context, items []string) error {
	pipe := f.client.Pipeline()

	for _, item := range items {
		indices := f.getIndices(item)
		for _, idx := range indices {
			pipe.SetBit(ctx, f.name, int64(idx), 1)
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}

// ExistsMany checks multiple items
//
// Returns a map of item to existence
func (f *Filter) ExistsMany(ctx context.Context, items []string) (map[string]bool, error) {
	result := make(map[string]bool)

	for _, item := range items {
		exists, err := f.Exists(ctx, item)
		if err != nil {
			return nil, err
		}
		result[item] = exists
	}

	return result, nil
}

// Reset clears all items from the filter
func (f *Filter) Reset(ctx context.Context) error {
	return f.client.Del(ctx, f.name).Err()
}

// Count returns the approximate number of items in the filter
func (f *Filter) Count(ctx context.Context) (uint64, error) {
	// Get all bits and estimate
	result, err := f.client.Get(ctx, f.name).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	// Estimate based on string length (8 bits per byte)
	// This is a rough approximation
	return uint64(len(result) * 8), nil
}

// Stats returns filter statistics
func (f *Filter) Stats(ctx context.Context) (Stats, error) {
	count, err := f.Count(ctx)
	if err != nil {
		return Stats{}, err
	}

	return Stats{
		Name:          f.name,
		ExpectedItems: f.expectedItems,
		FalsePositive: f.falsePositive,
		NumBits:       f.numBits,
		NumHashes:     f.numHashes,
		ItemCount:     count,
	}, nil
}

// Stats represents filter statistics
type Stats struct {
	Name          string  `json:"name"`
	ExpectedItems int     `json:"expected_items"`
	FalsePositive float64 `json:"false_positive_rate"`
	NumBits       uint64  `json:"num_bits"`
	NumHashes     uint64  `json:"num_hashes"`
	ItemCount     uint64  `json:"item_count"`
}

// getIndices returns the bitmap indices for an item
func (f *Filter) getIndices(item string) []uint64 {
	hash1 := fnv64a(item)
	hash2 := fnv64b(item)

	indices := make([]uint64, f.numHashes)
	for i := uint64(0); i < f.numHashes; i++ {
		indices[i] = (hash1 + i*hash2) % f.numBits
	}

	return indices
}

// optimalNumBits calculates optimal number of bits
func optimalNumBits(n int, p float64) uint64 {
	if n <= 0 || p <= 0 || p >= 1 {
		return uint64(n * 10)
	}
	m := -float64(n) * math.Log(p) / (math.Ln2 * math.Ln2)
	return uint64(math.Ceil(m))
}

// optimalNumHashes calculates optimal number of hash functions
func optimalNumHashes(n float64, m float64) uint64 {
	if n <= 0 || m <= 0 {
		return defaultHashes
	}
	k := (m / n) * math.Ln2
	return uint64(math.Ceil(k))
}

// fnv64a returns first hash value using FNV-1a
func fnv64a(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// fnv64b returns second hash value using FNV-1
func fnv64b(s string) uint64 {
	h := fnv.New64()
	h.Write([]byte(s))
	return h.Sum64()
}

// popcount returns number of set bits
func popcount(x int) int {
	count := 0
	for x > 0 {
		x &= x - 1
		count++
	}
	return count
}
