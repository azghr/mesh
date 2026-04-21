package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/azghr/mesh/logger"
)

var (
	ErrInvalidAPIKey  = errors.New("invalid API key")
	ErrAPIKeyExpired  = errors.New("API key expired")
	ErrAPIKeyRevoked  = errors.New("API key revoked")
	ErrAPIKeyNotFound = errors.New("API key not found")
)

type APIKey struct {
	ID          string
	Name        string
	KeyHash     string
	Permissions []string
	Roles       []string
	TenantID    string
	ExpiresAt   time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
	LastUsedAt  *time.Time
}

type APIKeyStore interface {
	Create(ctx context.Context, key *APIKey) error
	Get(ctx context.Context, id string) (*APIKey, error)
	GetByHash(ctx context.Context, keyHash string) (*APIKey, error)
	List(ctx context.Context, tenantID string) ([]*APIKey, error)
	Revoke(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	UpdateLastUsed(ctx context.Context, id string) error
}

type inMemoryAPIKeyStore struct {
	mu      sync.RWMutex
	keys    map[string]*APIKey
	keyHash map[string]string
}

func NewInMemoryAPIKeyStore() *inMemoryAPIKeyStore {
	return &inMemoryAPIKeyStore{
		keys:    make(map[string]*APIKey),
		keyHash: make(map[string]string),
	}
}

func (s *inMemoryAPIKeyStore) Create(ctx context.Context, key *APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key.ID == "" {
		key.ID = generateAPIKeyID()
	}
	if key.CreatedAt.IsZero() {
		key.CreatedAt = time.Now()
	}

	s.keys[key.ID] = key
	s.keyHash[key.KeyHash] = key.ID
	return nil
}

func (s *inMemoryAPIKeyStore) Get(ctx context.Context, id string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, ok := s.keys[id]
	if !ok {
		return nil, ErrAPIKeyNotFound
	}
	return key, nil
}

func (s *inMemoryAPIKeyStore) GetByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	s.mu.RLock()
	id, ok := s.keyHash[keyHash]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrAPIKeyNotFound
	}

	s.mu.RLock()
	key, ok := s.keys[id]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrAPIKeyNotFound
	}

	return key, nil
}

func (s *inMemoryAPIKeyStore) List(ctx context.Context, tenantID string) ([]*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*APIKey
	for _, key := range s.keys {
		if tenantID == "" || key.TenantID == tenantID {
			result = append(result, key)
		}
	}
	return result, nil
}

func (s *inMemoryAPIKeyStore) Revoke(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, ok := s.keys[id]
	if !ok {
		return ErrAPIKeyNotFound
	}

	now := time.Now()
	key.RevokedAt = &now
	delete(s.keyHash, key.KeyHash)
	return nil
}

func (s *inMemoryAPIKeyStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, ok := s.keys[id]
	if !ok {
		return ErrAPIKeyNotFound
	}

	delete(s.keyHash, key.KeyHash)
	delete(s.keys, id)
	return nil
}

func (s *inMemoryAPIKeyStore) UpdateLastUsed(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, ok := s.keys[id]
	if !ok {
		return ErrAPIKeyNotFound
	}

	now := time.Now()
	key.LastUsedAt = &now
	return nil
}

type APIKeyManager struct {
	store     APIKeyStore
	keyPrefix string
	keyLength int
}

func NewAPIKeyManager(store APIKeyKeyConfig) *APIKeyManager {
	if store.KeyLength < 16 {
		store.KeyLength = 32
	}
	if store.KeyPrefix == "" {
		store.KeyPrefix = "mesh_"
	}

	manager := &APIKeyManager{
		store:     store.Store,
		keyPrefix: store.KeyPrefix,
		keyLength: store.KeyLength,
	}

	if manager.store == nil {
		manager.store = NewInMemoryAPIKeyStore()
	}

	return manager
}

type APIKeyKeyConfig struct {
	Store     APIKeyStore
	KeyPrefix string
	KeyLength int
}

type CreatedAPIKey struct {
	ID        string
	RawKey    string
	ExpiresAt time.Time
}

func (m *APIKeyManager) CreateKey(ctx context.Context, name string, permissions, roles []string, tenantID string, ttl time.Duration) (*CreatedAPIKey, error) {
	rawKey := GenerateSecureToken(m.keyLength)
	keyHash := hashAPIKey(rawKey)

	expiresAt := time.Now().Add(ttl)
	if ttl <= 0 {
		expiresAt = time.Now().Add(365 * 24 * time.Hour)
	}

	apiKey := &APIKey{
		ID:          generateAPIKeyID(),
		Name:        name,
		KeyHash:     keyHash,
		Permissions: permissions,
		Roles:       roles,
		TenantID:    tenantID,
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now(),
	}

	if err := m.store.Create(ctx, apiKey); err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	logger.GetGlobal().Info("API key created", "name", name, "id", apiKey.ID)

	return &CreatedAPIKey{
		ID:        apiKey.ID,
		RawKey:    m.keyPrefix + rawKey,
		ExpiresAt: expiresAt,
	}, nil
}

func (m *APIKeyManager) ValidateKey(ctx context.Context, rawKey string) (*APIKey, error) {
	if len(rawKey) < len(m.keyPrefix) {
		return nil, ErrInvalidAPIKey
	}

	if !hasPrefixCaseInsensitive(rawKey, m.keyPrefix) {
		return nil, ErrInvalidAPIKey
	}

	keyPart := rawKey[len(m.keyPrefix):]
	keyHash := hashAPIKey(keyPart)

	key, err := m.store.GetByHash(ctx, keyHash)
	if err != nil {
		logger.GetGlobal().Warn("API key validation failed", "error", err)
		return nil, ErrInvalidAPIKey
	}

	if key.RevokedAt != nil {
		logger.GetGlobal().Warn("API key revoked", "key_id", key.ID)
		return nil, ErrAPIKeyRevoked
	}

	if time.Now().After(key.ExpiresAt) {
		logger.GetGlobal().Warn("API key expired", "key_id", key.ID)
		return nil, ErrAPIKeyExpired
	}

	_ = m.store.UpdateLastUsed(ctx, key.ID)

	return key, nil
}

func (m *APIKeyManager) RevokeKey(ctx context.Context, id string) error {
	logger.GetGlobal().Info("API key revoked", "id", id)
	return m.store.Revoke(ctx, id)
}

func (m *APIKeyManager) ListKeys(ctx context.Context, tenantID string) ([]*APIKey, error) {
	return m.store.List(ctx, tenantID)
}

func (m *APIKeyManager) GetKey(ctx context.Context, id string) (*APIKey, error) {
	return m.store.Get(ctx, id)
}

func generateAPIKeyID() string {
	return fmt.Sprintf("key_%s", GenerateSecureToken(16))
}

func hashAPIKey(key string) string {
	return fmt.Sprintf("%x", sha256Hash([]byte(key)))
}

func hasPrefixCaseInsensitive(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(s[:len(prefix)]), []byte(prefix)) == 1 ||
		subtle.ConstantTimeCompare([]byte(s[:len(prefix)]), []byte(toLower(prefix))) == 1
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

func sha256Hash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}
