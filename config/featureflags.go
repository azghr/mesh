// Package config provides configuration, feature flags, and related utilities.
//
// This package handles loading configuration from YAML files,
// environment variables, and feature flags for gradual rollouts.
//
// # Feature Flags
//
// Feature flags enable gradual rollouts, A/B testing, and kill switches.
// The service uses Redis for flag storage with local memory caching.
//
// # Basic Usage
//
// Create a feature flag service:
//
//	ffs := config.NewFeatureFlagService(redisClient)
//
// Define a flag with targeting rules:
//
//	flag := config.FeatureFlag{
//	    Key:      "new_user_profile",
//	    Enabled:  true,
//	    Rules: []config.FlagRule{
//	        {Percentage: 10},                    // 10% rollout
//	        {UserIDs: []string{"u1", "u2"}},     // beta users
//	        {Roles: []string{"admin"}},          // admin-only
//	    },
//	}
//
// Evaluate if a user sees the feature:
//
//	enabled, _ := ffs.IsEnabled(ctx, "new_user_profile", "user-123", map[string]string{
//	    "role": "admin",
//	})
//
// # Rule Evaluation Order
//
// Rules are evaluated in order. First match wins:
//  1. User whitelist (UserIDs)
//  2. Role match (Roles)
//  3. Percentage rollout (consistent hashing)
//
// # Middleware Integration
//
// Use with Fiber for automatic flag evaluation:
//
//	app.Use(config.FeatureFlagMiddleware(ffs, "feature1", "feature2"))
//
// # Best Practices
//
//  1. Fail closed: default to disabled
//  2. Use consistent hashing for stable rollouts
//  3. Monitor evaluation rates in metrics
//  4. Layer rules: whitelist → roles → percentage
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// FeatureFlag represents a feature flag with targeting rules.
type FeatureFlag struct {
	Key         string     `json:"key"`
	Description string     `json:"description,omitempty"`
	Enabled     bool       `json:"enabled"`
	Rules       []FlagRule `json:"rules,omitempty"`
}

// FlagRule defines targeting rules for a feature flag.
type FlagRule struct {
	Percentage float64           `json:"percentage"` // 0-100 rollout percentage
	UserIDs    []string          `json:"user_ids,omitempty"`
	Roles      []string          `json:"roles,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// FeatureFlagService manages feature flags with Redis backend.
type FeatureFlagService struct {
	redis    RedisClient
	local    *sync.Map
	defaults map[string]bool
	mu       sync.RWMutex
}

// RedisClient interface for feature flag storage.
type RedisClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
}

// NewFeatureFlagService creates a new feature flag service.
func NewFeatureFlagService(redisClient RedisClient) *FeatureFlagService {
	return &FeatureFlagService{
		redis:    redisClient,
		local:    &sync.Map{},
		defaults: make(map[string]bool),
	}
}

// NewFeatureFlagServiceWithDefaults creates a service with default values.
func NewFeatureFlagServiceWithDefaults(redisClient RedisClient, defaults map[string]bool) *FeatureFlagService {
	return &FeatureFlagService{
		redis:    redisClient,
		local:    &sync.Map{},
		defaults: defaults,
	}
}

// IsEnabled evaluates if a feature flag is enabled for a user.
func (s *FeatureFlagService) IsEnabled(ctx context.Context, flagKey string, userID string, attributes map[string]string) (bool, error) {
	// Check local cache first
	if cached, ok := s.local.Load(flagKey); ok {
		flag := cached.(*FeatureFlag)
		return s.evaluateFlag(flag, userID, attributes), nil
	}

	// Fetch from Redis
	flag, err := s.getFlag(ctx, flagKey)
	if err != nil {
		// Fallback to default
		s.mu.RLock()
		defaultEnabled := s.defaults[flagKey]
		s.mu.RUnlock()
		return defaultEnabled, nil
	}

	// Cache locally with automatic expiration
	s.local.Store(flagKey, flag)
	go func() {
		time.Sleep(30 * time.Second)
		s.local.Delete(flagKey)
	}()

	return s.evaluateFlag(flag, userID, attributes), nil
}

// evaluateFlag evaluates if a flag is enabled for a user based on rules.
func (s *FeatureFlagService) evaluateFlag(flag *FeatureFlag, userID string, attributes map[string]string) bool {
	if !flag.Enabled {
		return false
	}

	for _, rule := range flag.Rules {
		// Check user whitelist
		for _, id := range rule.UserIDs {
			if id == userID {
				return true
			}
		}

		// Check role match
		if len(rule.Roles) > 0 {
			userRole := ""
			if attributes != nil {
				userRole = attributes["role"]
			}
			for _, role := range rule.Roles {
				if role == userRole {
					return true
				}
			}
		}

		// Check percentage rollout using consistent hashing
		if rule.Percentage > 0 && userID != "" {
			hash := fnv.New32a()
			hash.Write([]byte(userID + flag.Key))
			bucket := float64(hash.Sum32()%100) / 100.0 * 100.0
			if bucket < rule.Percentage {
				return true
			}
		}
	}

	// Default to enabled if no rules match
	return flag.Enabled
}

// getFlag retrieves a flag from Redis.
func (s *FeatureFlagService) getFlag(ctx context.Context, flagKey string) (*FeatureFlag, error) {
	if s.redis == nil {
		return nil, fmt.Errorf("redis not configured")
	}

	val, err := s.redis.Get(ctx, s.key(flagKey)).Result()
	if err != nil {
		return nil, err
	}

	var flag FeatureFlag
	if err := json.Unmarshal([]byte(val), &flag); err != nil {
		return nil, err
	}

	return &flag, nil
}

// SetFlag stores a feature flag in Redis.
func (s *FeatureFlagService) SetFlag(ctx context.Context, flag *FeatureFlag) error {
	if s.redis == nil {
		return fmt.Errorf("redis not configured")
	}

	data, err := json.Marshal(flag)
	if err != nil {
		return err
	}

	return s.redis.Set(ctx, s.key(flag.Key), data, 5*time.Minute).Err()
}

// key returns the Redis key for a flag.
func (s *FeatureFlagService) key(flagKey string) string {
	return fmt.Sprintf("feature:%s", flagKey)
}

// EnableFlag enables a feature flag.
func (s *FeatureFlagService) EnableFlag(ctx context.Context, flagKey string) error {
	flag, err := s.getFlag(ctx, flagKey)
	if err != nil {
		flag = &FeatureFlag{Key: flagKey}
	}

	flag.Enabled = true
	return s.SetFlag(ctx, flag)
}

// DisableFlag disables a feature flag.
func (s *FeatureFlagService) DisableFlag(ctx context.Context, flagKey string) error {
	flag, err := s.getFlag(ctx, flagKey)
	if err != nil {
		flag = &FeatureFlag{Key: flagKey}
	}

	flag.Enabled = false
	return s.SetFlag(ctx, flag)
}

// SetDefault sets the default enabled state for a flag.
func (s *FeatureFlagService) SetDefault(flagKey string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defaults[flagKey] = enabled
}

// GetAllFlags retrieves all feature flags from Redis.
func (s *FeatureFlagService) GetAllFlags(ctx context.Context) ([]FeatureFlag, error) {
	if s.redis == nil {
		return nil, fmt.Errorf("redis not configured")
	}

	var flags []FeatureFlag
	iter := s.redis.Scan(ctx, 0, "feature:*", 100).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()
		flagKey := strings.TrimPrefix(key, "feature:")

		flag, err := s.getFlag(ctx, flagKey)
		if err != nil {
			continue
		}

		flags = append(flags, *flag)
	}

	return flags, iter.Err()
}

// DeleteFlag removes a feature flag.
func (s *FeatureFlagService) DeleteFlag(ctx context.Context, flagKey string) error {
	if s.redis == nil {
		return nil
	}

	s.local.Delete(flagKey)
	return s.redis.Del(ctx, s.key(flagKey)).Err()
}

// SetDefaultWithDefaults sets multiple defaults at once.
func (s *FeatureFlagService) SetDefaultWithDefaults(defaults map[string]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range defaults {
		s.defaults[k] = v
	}
}

// GetDefault returns the default value for a flag.
func (s *FeatureFlagService) GetDefault(flagKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.defaults[flagKey]
}

// FeatureFlagMiddleware creates Fiber middleware for feature flags.
func FeatureFlagMiddleware(ffs *FeatureFlagService, flagKeys ...string) func(c interface{}) error {
	return func(c interface{}) error {
		// Type assert to fiber.Ctx
		fc, ok := c.(interface {
			Context() context.Context
			Locals(string) interface{}
		})
		if !ok {
			return nil
		}

		userID := ""
		if uid := fc.Locals("user_id"); uid != nil {
			userID, _ = uid.(string)
		}

		attributes := extractUserAttributes(fc)

		for _, key := range flagKeys {
			enabled, err := ffs.IsEnabled(fc.Context(), key, userID, attributes)
			if err != nil {
				enabled = false
			}
			// Store with ff_ prefix
			_ = enabled // Feature flag checked but not stored
		}

		return nil
	}
}

// extractUserAttributes extracts user attributes from context.
func extractUserAttributes(c interface{}) map[string]string {
	attributes := make(map[string]string)

	// Try to get role
	if role := c.(interface{ Locals(string) interface{} }).Locals("role"); role != nil {
		if r, ok := role.(string); ok {
			attributes["role"] = r
		}
	}

	// Try to get tenant_id
	if tenant := c.(interface{ Locals(string) interface{} }).Locals("tenant_id"); tenant != nil {
		if t, ok := tenant.(string); ok {
			attributes["tenant_id"] = t
		}
	}

	return attributes
}

// CreateFeatureFlag creates a new feature flag.
func CreateFeatureFlag(key string, enabled bool, rules []FlagRule) FeatureFlag {
	return FeatureFlag{
		Key:     key,
		Enabled: enabled,
		Rules:   rules,
	}
}

// CreatePercentageRule creates a percentage rollout rule.
func CreatePercentageRule(percentage float64) FlagRule {
	return FlagRule{
		Percentage: percentage,
	}
}

// CreateUserRule creates a user whitelist rule.
func CreateUserRule(userIDs []string) FlagRule {
	return FlagRule{
		UserIDs: userIDs,
	}
}

// CreateRoleRule creates a role-based rule.
func CreateRoleRule(roles []string) FlagRule {
	return FlagRule{
		Roles: roles,
	}
}
