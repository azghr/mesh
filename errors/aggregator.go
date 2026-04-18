// Package errors provides distributed error aggregation for monitoring.
//
// This package aggregates and groups similar errors for monitoring:
//
//   - Track errors by type and code
//   - Group similar errors together
//   - Trigger alerts when thresholds are exceeded
//
// Quick Example:
//
//	aggregator := errors.NewAggregator(
//	    errors.WithWindow(5 * time.Minute),
//	    errors.WithMinOccurrences(10),
//	)
//
//	aggregator.Record(err)
//	aggregator.Record(err)
//	aggregator.Record(err)
//
//	// Get aggregated errors
//	grouped := aggregator.GetGroupedErrors()
package errors

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type (
	AggregatorConfig struct {
		Window         time.Duration
		GroupBy        []string
		MinOccurrences int
		EnableAlerts   bool
		AlertHandler   func(ErrorGroup)
	}

	Aggregator struct {
		config      *AggregatorConfig
		mu          sync.RWMutex
		groups      map[string]*ErrorGroup
		alerts      map[string]*AlertState
		lastCleanup time.Time
	}

	ErrorGroup struct {
		Type          ErrorType
		Code          string
		Count         int
		FirstOccurred time.Time
		LastOccurred  time.Time
		SampleErrors  []error
	}

	AlertState struct {
		LastAlert  time.Time
		AlertCount int
		Threshold  int
	}

	GroupFilter struct {
		ErrType    ErrorType
		Code       string
		Threshold  int
		TimeWindow time.Duration
	}

	AggregatorOption func(*AggregatorConfig)
)

const maxSampleErrors = 5

func WithWindow(d time.Duration) AggregatorOption {
	return func(c *AggregatorConfig) {
		c.Window = d
	}
}

func WithGroupBy(fields ...string) AggregatorOption {
	return func(c *AggregatorConfig) {
		c.GroupBy = fields
	}
}

func WithMinOccurrences(n int) AggregatorOption {
	return func(c *AggregatorConfig) {
		c.MinOccurrences = n
	}
}

func WithAlerts(enabled bool, handler func(ErrorGroup)) AggregatorOption {
	return func(c *AggregatorConfig) {
		c.EnableAlerts = enabled
		c.AlertHandler = handler
	}
}

func NewAggregator(opts ...AggregatorOption) *Aggregator {
	cfg := &AggregatorConfig{
		Window:         5 * time.Minute,
		GroupBy:        []string{"type", "code"},
		MinOccurrences: 10,
		EnableAlerts:   false,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return &Aggregator{
		config:      cfg,
		groups:      make(map[string]*ErrorGroup),
		alerts:      make(map[string]*AlertState),
		lastCleanup: time.Now(),
	}
}

func (a *Aggregator) Record(err error) {
	if err == nil {
		return
	}

	appErr, ok := err.(*AppError)
	if !ok {
		appErr = Wrap(err, ErrorTypeDatabase, "unknown", err.Error())
	}

	key := a.groupKey(appErr)

	a.mu.Lock()
	defer a.mu.Unlock()

	group, exists := a.groups[key]
	if !exists {
		group = &ErrorGroup{
			Type:          appErr.Type,
			Code:          string(appErr.Type) + ":" + appErr.Code,
			FirstOccurred: time.Now(),
		}
		a.groups[key] = group
	}

	group.Count++
	group.LastOccurred = time.Now()

	if len(group.SampleErrors) < maxSampleErrors {
		group.SampleErrors = append(group.SampleErrors, err)
	}

	if a.config.EnableAlerts && a.config.AlertHandler != nil {
		a.checkAlert(group)
	}

	a.cleanupIfNeeded()
}

func (a *Aggregator) groupKey(err *AppError) string {
	var parts []string
	for _, field := range a.config.GroupBy {
		switch field {
		case "type":
			parts = append(parts, string(err.Type))
		case "code":
			parts = append(parts, err.Code)
		case "message":
			parts = append(parts, truncateMessage(err.Message, 50))
		default:
			parts = append(parts, "unknown")
		}
	}
	return strings.Join(parts, ":")
}

func truncateMessage(msg string, maxLen int) string {
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen] + "..."
}

func (a *Aggregator) GetGroupedErrors() []ErrorGroup {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []ErrorGroup
	for _, group := range a.groups {
		if group.Count >= a.config.MinOccurrences {
			result = append(result, *group)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	return result
}

func (a *Aggregator) GetErrorCount(errType ErrorType) int {
	a.mu.RLock()
	defer a.mu.RUnlock()

	count := 0
	for _, group := range a.groups {
		if group.Type == errType {
			count += group.Count
		}
	}
	return count
}

func (a *Aggregator) GetErrorCountByCode(code string) int {
	a.mu.RLock()
	defer a.mu.RUnlock()

	count := 0
	for _, group := range a.groups {
		if group.Code == code {
			count += group.Count
		}
	}
	return count
}

func (a *Aggregator) EnableAlert(filter GroupFilter, handler func(ErrorGroup)) {
	a.config.EnableAlerts = true
	a.config.AlertHandler = handler

	a.mu.Lock()
	if a.alerts == nil {
		a.alerts = make(map[string]*AlertState)
	}
	a.alerts[string(filter.ErrType)+":"+filter.Code] = &AlertState{
		Threshold: filter.Threshold,
	}
	a.mu.Unlock()
}

func (a *Aggregator) checkAlert(group *ErrorGroup) {
	filterKey := string(group.Type) + ":" + group.Code

	a.mu.RLock()
	alertState, exists := a.alerts[filterKey]
	a.mu.RUnlock()

	if !exists {
		return
	}

	if group.Count >= alertState.Threshold {
		now := time.Now()

		if alertState.LastAlert.IsZero() ||
			now.Sub(alertState.LastAlert) > a.config.Window {

			go a.config.AlertHandler(*group)

			a.mu.Lock()
			alertState.LastAlert = now
			alertState.AlertCount++
			a.mu.Unlock()
		}
	}
}

func (a *Aggregator) cleanupIfNeeded() {
	now := time.Now()
	if now.Sub(a.lastCleanup) < a.config.Window {
		return
	}

	a.lastCleanup = now

	windowStart := now.Add(-a.config.Window)

	for key, group := range a.groups {
		if group.LastOccurred.Before(windowStart) {
			delete(a.groups, key)
		}
	}
}

func (a *Aggregator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.groups = make(map[string]*ErrorGroup)
	a.alerts = make(map[string]*AlertState)
}

func (a *Aggregator) Stats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	totalErrors := 0
	groupsByType := make(map[ErrorType]int)

	for _, group := range a.groups {
		totalErrors += group.Count
		groupsByType[group.Type] += group.Count
	}

	return map[string]interface{}{
		"total_errors": totalErrors,
		"groups":       len(a.groups),
		"by_type":      groupsByType,
	}
}

func (a *Aggregator) GroupErrorsByType() map[ErrorType][]ErrorGroup {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[ErrorType][]ErrorGroup)

	for _, group := range a.groups {
		result[group.Type] = append(result[group.Type], *group)
	}

	return result
}

func (a *Aggregator) GetTopErrors(limit int) []ErrorGroup {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var all []ErrorGroup
	for _, group := range a.groups {
		all = append(all, *group)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Count > all[j].Count
	})

	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}

	return all
}

func (a *Aggregator) String() string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var parts []string
	for _, group := range a.groups {
		parts = append(parts, fmt.Sprintf("%s:%d", group.Type, group.Count))
	}
	return strings.Join(parts, " ")
}
