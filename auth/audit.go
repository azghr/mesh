package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/azghr/mesh/logger"
)

type AuditEventType string

const (
	EventLogin            AuditEventType = "login"
	EventLogout           AuditEventType = "logout"
	EventTokenRefresh     AuditEventType = "token_refresh"
	EventTokenInvalidate  AuditEventType = "token_invalidate"
	EventPermissionDenied AuditEventType = "permission_denied"
	EventRoleChange       AuditEventType = "role_change"
	EventAPIKeyCreated    AuditEventType = "api_key_created"
	EventAPIKeyRevoked    AuditEventType = "api_key_revoked"
	EventAPIKeyUsed       AuditEventType = "api_key_used"
	EventAuthFailure      AuditEventType = "auth_failure"
)

type AuditEvent struct {
	Type      AuditEventType         `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	UserID    string                 `json:"user_id,omitempty"`
	TenantID  string                 `json:"tenant_id,omitempty"`
	IPAddress string                 `json:"ip_address,omitempty"`
	UserAgent string                 `json:"user_agent,omitempty"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type AuditLogger interface {
	Log(ctx context.Context, event AuditEvent)
	Query(ctx context.Context, filter AuditFilter) ([]AuditEvent, error)
}

type AuditFilter struct {
	UserID    string
	TenantID  string
	EventType AuditEventType
	StartTime time.Time
	EndTime   time.Time
	Limit     int
}

type inMemoryAuditLogger struct {
	mu      sync.RWMutex
	events  []AuditEvent
	maxSize int
}

func NewInMemoryAuditLogger(maxSize int) *inMemoryAuditLogger {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &inMemoryAuditLogger{
		events:  make([]AuditEvent, 0, maxSize),
		maxSize: maxSize,
	}
}

func (l *inMemoryAuditLogger) Log(ctx context.Context, event AuditEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.events = append(l.events, event)
	if len(l.events) > l.maxSize {
		l.events = l.events[len(l.events)-l.maxSize:]
	}

	logger.GetGlobal().Info("audit event",
		"type", event.Type,
		"user_id", event.UserID,
		"success", event.Success,
	)
}

func (l *inMemoryAuditLogger) Query(ctx context.Context, filter AuditFilter) ([]AuditEvent, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []AuditEvent
	for _, event := range l.events {
		if filter.UserID != "" && event.UserID != filter.UserID {
			continue
		}
		if filter.TenantID != "" && event.TenantID != filter.TenantID {
			continue
		}
		if filter.EventType != "" && event.Type != filter.EventType {
			continue
		}
		if !filter.StartTime.IsZero() && event.Timestamp.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && event.Timestamp.After(filter.EndTime) {
			continue
		}
		result = append(result, event)
	}

	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[len(result)-filter.Limit:]
	}

	return result, nil
}

type AuditRecorder struct {
	logger AuditLogger
}

func NewAuditRecorder(logger AuditLogger) *AuditRecorder {
	if logger == nil {
		logger = NewInMemoryAuditLogger(10000)
	}
	return &AuditRecorder{logger: logger}
}

func (r *AuditRecorder) RecordLogin(ctx context.Context, userID, tenantID, ipAddress, userAgent string, success bool, err error) {
	r.logger.Log(ctx, AuditEvent{
		Type:      EventLogin,
		UserID:    userID,
		TenantID:  tenantID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   success,
		Error:     errorToString(err),
	})
}

func (r *AuditRecorder) RecordLogout(ctx context.Context, userID, tenantID string) {
	r.logger.Log(ctx, AuditEvent{
		Type:     EventLogout,
		UserID:   userID,
		TenantID: tenantID,
		Success:  true,
	})
}

func (r *AuditRecorder) RecordTokenRefresh(ctx context.Context, userID, tenantID string) {
	r.logger.Log(ctx, AuditEvent{
		Type:     EventTokenRefresh,
		UserID:   userID,
		TenantID: tenantID,
		Success:  true,
	})
}

func (r *AuditRecorder) RecordPermissionDenied(ctx context.Context, userID, permission, resource string) {
	r.logger.Log(ctx, AuditEvent{
		Type:     EventPermissionDenied,
		UserID:   userID,
		Success:  false,
		Metadata: map[string]interface{}{"permission": permission, "resource": resource},
	})
}

func (r *AuditRecorder) RecordAPIKeyUsed(ctx context.Context, keyID, tenantID, ipAddress string, success bool) {
	r.logger.Log(ctx, AuditEvent{
		Type:      EventAPIKeyUsed,
		UserID:    keyID,
		TenantID:  tenantID,
		IPAddress: ipAddress,
		Success:   success,
	})
}

func (r *AuditRecorder) RecordAuthFailure(ctx context.Context, userID, reason, ipAddress string) {
	r.logger.Log(ctx, AuditEvent{
		Type:      EventAuthFailure,
		UserID:    userID,
		Success:   false,
		Error:     reason,
		IPAddress: ipAddress,
	})
}

func (r *AuditRecorder) Query(ctx context.Context, filter AuditFilter) ([]AuditEvent, error) {
	return r.logger.Query(ctx, filter)
}

func errorToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (e AuditEvent) String() string {
	b, _ := json.Marshal(e)
	return string(b)
}

func (e AuditEvent) ToJSON() (string, error) {
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal audit event: %w", err)
	}
	return string(b), nil
}
