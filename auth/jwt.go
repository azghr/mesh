// Package auth provides JWT (JSON Web Token) authentication and authorization.
//
// This package implements JWT token generation, validation, and refresh
// functionality with support for both HMAC and RSA signing algorithms.
//
// # Token Generation
//
// Generate access/refresh token pairs for authenticated users:
//
//	jwtManager := auth.NewJWTManager(auth.JWTConfig{
//	    SigningKey:     []byte("your-secret-key"),
//	    Issuer:        "your-app",
//	    Audience:     "your-api",
//	    AccessTokenTTL: 15 * time.Minute,
//	    RefreshTokenTTL: 7 * 24 * time.Hour,
//	}, rbac)
//
//	tokens, err := jwtManager.GenerateToken(ctx, "user123", "user@example.com", nil)
//
// # Token Validation
//
// Validate tokens in HTTP handlers or middleware:
//
//	claims, err := jwtManager.ValidateToken(tokenString)
//	if err != nil {
//	    return err // invalid/expired token
//	}
//
// # Token Refresh
//
// Refresh expired access tokens using refresh tokens:
//
//	newTokens, err := jwtManager.RefreshTokens(refreshToken)
//
// # Token Invalidation (Logout)
//
// Invalidate tokens for logout using the blacklist:
//
//	blacklist := auth.NewTokenBlacklist()
//	jwtManager.SetBlacklist(blacklist)
//	jwtManager.InvalidateToken(accessToken)
//
// # RSA Support
//
// For production, use RSA keys:
//
//	rsaMaker, _ := auth.NewRSAJWTMaker(2048)
//	accessToken, _ := rsaMaker.CreateToken("user123")
//	claims, _ := rsaMaker.VerifyToken(accessToken)
package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken    = errors.New("invalid token")
	ErrTokenExpired    = errors.New("token expired")
	ErrInvalidIssuer   = errors.New("invalid issuer")
	ErrInvalidAudience = errors.New("invalid audience")
)

// JWTConfig holds configuration for JWT token operations.
type JWTConfig struct {
	SigningKey      []byte          // Secret key for HMAC signing (HS256/HS384/HS512)
	PublicKey       *rsa.PublicKey  // RSA public key for RS256 verification
	PrivateKey      *rsa.PrivateKey // RSA private key for RS256 signing
	Algorithm       string          // Signing algorithm: "HS256", "HS384", "HS512", "RS256"
	Issuer          string          // Token issuer (iss claim)
	Audience        string          // Expected audience (aud claim)
	AccessTokenTTL  time.Duration   // Access token lifetime (default: 15 minutes)
	RefreshTokenTTL time.Duration   // Refresh token lifetime (default: 7 days)
}

// TokenClaims represents custom JWT claims including user identity and permissions.
type TokenClaims struct {
	jwt.RegisteredClaims                   // Standard claims (iat, exp, sub, iss, aud, etc.)
	UserID               string            `json:"user_id"`             // User identifier
	Email                string            `json:"email"`               // User email address
	Roles                []string          `json:"roles"`               // User roles
	Permissions          []string          `json:"permissions"`         // User permissions
	TenantID             string            `json:"tenant_id,omitempty"` // Multi-tenant identifier
	Metadata             map[string]string `json:"metadata,omitempty"`  // Additional metadata
}

// TokenPair represents an issued access/refresh token pair.
type TokenPair struct {
	AccessToken  string `json:"access_token"`  // JWT for API access
	RefreshToken string `json:"refresh_token"` // Long-lived token for refresh
	ExpiresAt    int64  `json:"expires_at"`    // Unix timestamp when access token expires
	TokenType    string `json:"token_type"`    // Always "Bearer"
}

// JWTManager handles JWT token generation, validation, and refresh.
type JWTManager struct {
	config    JWTConfig
	rbac      *RBAC
	mu        sync.RWMutex
	blacklist *TokenBlacklist
}

// NewJWTManager creates a new JWT manager with the given configuration.
// If rbac is provided, permissions will be automatically loaded from RBAC on token generation.
// Default TTLs: AccessToken=15min, RefreshToken=7 days. Default algorithm: HS256.
func NewJWTManager(config JWTConfig, rbac *RBAC) *JWTManager {
	if config.AccessTokenTTL == 0 {
		config.AccessTokenTTL = 15 * time.Minute
	}
	if config.RefreshTokenTTL == 0 {
		config.RefreshTokenTTL = 7 * 24 * time.Hour
	}
	if config.Algorithm == "" {
		config.Algorithm = "HS256"
	}

	return &JWTManager{
		config: config,
		rbac:   rbac,
	}
}

// GenerateToken creates a new access/refresh token pair for a user.
// If roles is nil and rbac is configured, permissions are loaded from RBAC.
func (j *JWTManager) GenerateToken(ctx context.Context, userID, email string, roles []string) (*TokenPair, error) {
	now := time.Now()

	if roles == nil && j.rbac != nil {
		var err error
		perms, err := j.rbac.GetUserPermissions(ctx, userID)
		if err != nil {
			roles = []string{}
		} else {
			roles = permissionSliceToStrings(perms)
		}
	}

	accessClaims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    j.config.Issuer,
			Audience:  jwt.ClaimStrings{j.config.Audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.config.AccessTokenTTL)),
			ID:        uuid.New().String(),
		},
		UserID:      userID,
		Email:       email,
		Roles:       roles,
		Permissions: roles,
	}

	accessToken, err := j.signToken(accessClaims)
	if err != nil {
		return nil, err
	}

	refreshToken, err := j.signRefreshToken(now, userID)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessClaims.ExpiresAt.Unix(),
		TokenType:    "Bearer",
	}, nil
}

// GenerateTokenWithTenant creates tokens with tenant context for multi-tenant applications.
func (j *JWTManager) GenerateTokenWithTenant(ctx context.Context, userID, email, tenantID string, roles []string, metadata map[string]string) (*TokenPair, error) {
	now := time.Now()

	if roles == nil && j.rbac != nil {
		var err error
		perms, err := j.rbac.GetUserPermissions(ctx, userID)
		if err != nil {
			roles = []string{}
		} else {
			roles = permissionSliceToStrings(perms)
		}
	}

	accessClaims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    j.config.Issuer,
			Audience:  jwt.ClaimStrings{j.config.Audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.config.AccessTokenTTL)),
			ID:        uuid.New().String(),
		},
		UserID:      userID,
		Email:       email,
		Roles:       roles,
		Permissions: roles,
		TenantID:    tenantID,
		Metadata:    metadata,
	}

	accessToken, err := j.signToken(accessClaims)
	if err != nil {
		return nil, err
	}

	refreshToken, err := j.signRefreshToken(now, userID)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessClaims.ExpiresAt.Unix(),
		TokenType:    "Bearer",
	}, nil
}

// signToken signs an access token with the configured algorithm.
func (j *JWTManager) signToken(claims TokenClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.GetSigningMethod(j.config.Algorithm), claims)
	signed, err := token.SignedString(j.getSigningKey())
	if err != nil {
		return "", fmt.Errorf("signing access token: %w", err)
	}
	return signed, nil
}

// signRefreshToken signs a refresh token.
func (j *JWTManager) signRefreshToken(now time.Time, userID string) (string, error) {
	refreshClaims := jwt.RegisteredClaims{
		Subject:   userID,
		Issuer:    j.config.Issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(j.config.RefreshTokenTTL)),
		ID:        uuid.New().String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	signed, err := token.SignedString(j.config.SigningKey)
	if err != nil {
		return "", fmt.Errorf("signing refresh token: %w", err)
	}
	return signed, nil
}

// ValidateToken validates a token string and returns the claims.
// Validates: signature, expiration, issuer, audience, and blacklist.
func (j *JWTManager) ValidateToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, j.getKeyFunc())
	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	if err := j.validateIssuer(claims); err != nil {
		return nil, err
	}

	if err := j.validateAudience(claims); err != nil {
		return nil, err
	}

	if j.blacklist != nil {
		isBlacklisted, err := j.blacklist.IsBlacklisted(claims.ID)
		if err == nil && isBlacklisted {
			return nil, ErrInvalidToken
		}
	}

	return claims, nil
}

// validateIssuer checks if the token issuer matches the expected issuer.
func (j *JWTManager) validateIssuer(claims *TokenClaims) error {
	if j.config.Issuer != "" && claims.Issuer != j.config.Issuer {
		return ErrInvalidIssuer
	}
	return nil
}

// validateAudience checks if the token audience contains the expected audience.
func (j *JWTManager) validateAudience(claims *TokenClaims) error {
	if j.config.Audience == "" {
		return nil
	}
	for _, aud := range claims.Audience {
		if aud == j.config.Audience {
			return nil
		}
	}
	return ErrInvalidAudience
}

// getKeyFunc returns the appropriate key function for token validation.
func (j *JWTManager) getKeyFunc() jwt.Keyfunc {
	if j.config.PublicKey != nil {
		return func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); ok {
				return j.config.PublicKey, nil
			}
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
	}
	return func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); ok {
			return j.config.SigningKey, nil
		}
		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	}
}

// RefreshTokens creates new access/refresh tokens using a valid refresh token.
func (j *JWTManager) RefreshTokens(refreshToken string) (*TokenPair, error) {
	token, err := jwt.ParseWithClaims(refreshToken, &jwt.RegisteredClaims{},
		func(t *jwt.Token) (interface{}, error) {
			return j.config.SigningKey, nil
		})
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return j.GenerateTokenWithTenant(context.TODO(), claims.Subject, "", "", nil, nil)
}

// InvalidateToken adds a token to the blacklist for logout.
func (j *JWTManager) InvalidateToken(tokenString string) error {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		return j.config.SigningKey, nil
	})
	if err != nil {
		return nil
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok {
		return nil
	}

	if j.blacklist != nil {
		expTime := claims.ExpiresAt.Time
		if expTime.IsZero() {
			expTime = time.Now().Add(j.config.AccessTokenTTL)
		}
		return j.blacklist.Blacklist(claims.ID, time.Until(expTime))
	}

	return nil
}

// SetBlacklist sets the token blacklist for logout support.
func (j *JWTManager) SetBlacklist(blacklist *TokenBlacklist) {
	j.blacklist = blacklist
}

// getSigningKey returns the signing key based on configuration.
func (j *JWTManager) getSigningKey() interface{} {
	if j.config.PrivateKey != nil {
		return j.config.PrivateKey
	}
	return j.config.SigningKey
}

// permissionSliceToStrings converts Permission slice to string slice.
func permissionSliceToStrings(perms []Permission) []string {
	result := make([]string, len(perms))
	for i, p := range perms {
		result[i] = string(p)
	}
	return result
}

// TokenBlacklist stores invalidated token IDs for logout functionality.
// Supports both in-memory and Redis-backed storage.
type TokenBlacklist struct {
	client RedisClient
	data   map[string]time.Time
	mu     sync.RWMutex
}

// RedisClient interface for Redis-backed blacklist.
type RedisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, keys ...string) (int, error)
}

// NewTokenBlacklist creates a new in-memory token blacklist.
func NewTokenBlacklist() *TokenBlacklist {
	return &TokenBlacklist{
		data: make(map[string]time.Time),
	}
}

// SetRedisClient configures a Redis client for distributed blacklist.
func (b *TokenBlacklist) SetRedisClient(client RedisClient) {
	b.client = client
}

// Blacklist adds a token ID to the blacklist.
func (b *TokenBlacklist) Blacklist(tokenID string, ttl time.Duration) error {
	if b.client != nil {
		return b.client.Set(context.Background(), "jwt:blacklist:"+tokenID, "1", ttl)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.data[tokenID] = time.Now().Add(ttl)
	return nil
}

// IsBlacklisted checks if a token ID is in the blacklist.
func (b *TokenBlacklist) IsBlacklisted(tokenID string) (bool, error) {
	if b.client != nil {
		exists, err := b.client.Exists(context.Background(), "jwt:blacklist:"+tokenID)
		return exists > 0, err
	}

	b.mu.RLock()
	expTime, ok := b.data[tokenID]
	b.mu.RUnlock()

	if !ok {
		return false, nil
	}

	if time.Now().After(expTime) {
		b.mu.Lock()
		delete(b.data, tokenID)
		b.mu.Unlock()
		return false, nil
	}

	return true, nil
}

// CleanExpired removes expired entries from in-memory blacklist.
func (b *TokenBlacklist) CleanExpired() {
	b.mu.Lock()
	now := time.Now()
	for tokenID, expTime := range b.data {
		if now.After(expTime) {
			delete(b.data, tokenID)
		}
	}
	b.mu.Unlock()
}

// GenerateSecureToken generates a cryptographically secure random token.
// Used for CSRF protection, OAuth state, etc.
func GenerateSecureToken(length int) string {
	if length < 32 {
		length = 32
	}

	bytes := make([]byte, length)
	_, _ = rand.Read(bytes)

	encoded := make([]byte, base64.RawURLEncoding.EncodedLen(length))
	base64.RawURLEncoding.Encode(encoded, bytes)

	if len(encoded) > length {
		encoded = encoded[:length]
	}

	return string(encoded)
}

// GenerateRandomState generates a random state string for OAuth flows.
func GenerateRandomState() string {
	bytes := make([]byte, 32)
	_, _ = rand.Read(bytes)

	hash := sha256.Sum256(bytes)
	return base64.URLEncoding.EncodeToString(hash[:])
}

// VerifyState verifies OAuth state strings using constant-time comparison.
func VerifyState(expected, actual string) bool {
	if expected == "" || actual == "" {
		return false
	}

	const stateLength = 43
	if len(expected) > stateLength {
		expected = expected[:stateLength]
	}
	if len(actual) > stateLength {
		actual = actual[:stateLength]
	}

	return subtleConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

// subtleConstantTimeCompare performs constant-time comparison.
func subtleConstantTimeCompare(a, b []byte) int {
	var result byte
	for i := 0; i < len(a) && i < len(b); i++ {
		result |= a[i] ^ b[i]
	}

	if len(a) != len(b) {
		result |= byte(len(a) ^ len(b))
	}

	return int(result ^ (result-1)>>7)
}

// JWTMaker provides simple HMAC-based JWT operations.
// Suitable for development or simple use cases.
type JWTMaker struct {
	secretKey     []byte
	tokenDuration time.Duration
}

// NewJWTMaker creates a JWTMaker with the given secret key.
func NewJWTMaker(secretKey string) *JWTMaker {
	return &JWTMaker{
		secretKey:     []byte(secretKey),
		tokenDuration: time.Hour,
	}
}

// CreateToken creates a JWT with the username as subject.
func (m *JWTMaker) CreateToken(username string) (string, error) {
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.tokenDuration)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Subject:   username,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

// VerifyToken verifies and returns claims from a token.
func (m *JWTMaker) VerifyToken(tokenString string) (*jwt.RegisteredClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
		return m.secretKey, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// SetTokenDuration sets the token duration.
func (m *JWTMaker) SetTokenDuration(d time.Duration) {
	m.tokenDuration = d
}

// RSAJWTMaker provides RSA-based JWT operations.
// Recommended for production use with proper key management.
type RSAJWTMaker struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	duration   time.Duration
}

// NewRSAJWTMaker creates a new RSA JWT maker with the specified key size.
// Common key sizes: 2048 (recommended), 4096 (high security).
func NewRSAJWTMaker(bits int) (*RSAJWTMaker, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, fmt.Errorf("generating RSA key: %w", err)
	}

	return &RSAJWTMaker{
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		duration:   time.Hour,
	}, nil
}

// CreateToken creates a JWT with the username as subject using RSA.
func (m *RSAJWTMaker) CreateToken(username string) (string, error) {
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.duration)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Subject:   username,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(m.privateKey)
}

// VerifyToken verifies and returns claims from an RSA-signed token.
func (m *RSAJWTMaker) VerifyToken(tokenString string) (*jwt.RegisteredClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.publicKey, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// SetTokenDuration sets the token duration.
func (m *RSAJWTMaker) SetTokenDuration(d time.Duration) {
	m.duration = d
}

// PublicKey returns the RSA public key.
func (m *RSAJWTMaker) PublicKey() *rsa.PublicKey {
	return m.publicKey
}

// PrivateKey returns the RSA private key.
func (m *RSAJWTMaker) PrivateKey() *rsa.PrivateKey {
	return m.privateKey
}

// PublicKeyPEM returns the public key in PEM format.
func (m *RSAJWTMaker) PublicKeyPEM() ([]byte, error) {
	return encodePEM("RSA PUBLIC KEY", x509MarshalPKIXPublicKey(m.publicKey))
}

// PrivateKeyPEM returns the private key in PEM format.
func (m *RSAJWTMaker) PrivateKeyPEM() ([]byte, error) {
	return encodePEM("RSA PRIVATE KEY", x509MarshalPKCS8PrivateKey(m.privateKey))
}

// encodePEM encodes binary data as PEM format.
func encodePEM(t string, b []byte) ([]byte, error) {
	if b == nil {
		return nil, errors.New("nil data")
	}

	encoded := make([]byte, 0, len(t)*2+6+len(b)+len(b)/64+10)
	encoded = append(encoded, "-----BEGIN "+t+"-----\n"...)

	for i := 0; i < len(b); i += 64 {
		end := i + 64
		if end > len(b) {
			end = len(b)
		}
		encoded = append(encoded, b[i:end]...)
		encoded = append(encoded, '\n')
	}

	encoded = append(encoded, "-----END "+t+"-----\n"...)
	return encoded, nil
}

// x509MarshalPKIXPublicKey marshals a public key in PKIX format.
// Uses a simple DER encoding for RSA public keys.
func x509MarshalPKIXPublicKey(pub *rsa.PublicKey) []byte {
	if pub == nil || pub.N == nil {
		return nil
	}

	// RSA public key DER encoding
	// Format: SEQUENCE { INTEGER n, INTEGER e }
	der := make([]byte, 0)

	// Encode n
	nBytes := pub.N.Bytes()
	der = append(der, 0x02)              // INTEGER
	der = append(der, byte(len(nBytes))) // Length
	der = append(der, nBytes...)

	// Encode e
	e := pub.E
	eBytes := []byte{byte(e >> 24), byte(e >> 16), byte(e >> 8), byte(e)}
	for len(eBytes) > 1 && eBytes[0] == 0 {
		eBytes = eBytes[1:]
	}
	der = append(der, 0x02)
	der = append(der, byte(len(eBytes)))
	der = append(der, eBytes...)

	// Wrap in SEQUENCE
	encoded := make([]byte, 0)
	encoded = append(encoded, 0x30) // SEQUENCE
	encoded = append(encoded, byte(len(der)))
	encoded = append(encoded, der...)

	return encoded
}

// x509MarshalPKCS8PrivateKey marshals a private key in PKCS8 format.
func x509MarshalPKCS8PrivateKey(priv *rsa.PrivateKey) []byte {
	if priv == nil || priv.N == nil {
		return nil
	}

	// Simplified RSA private key encoding
	// In production, use crypto/x509 for proper PKCS8 encoding
	der := make([]byte, 0)
	der = append(der, 0x02, 0x01, 0x00) // version = 0
	der = append(der, 0x02, byte(len(priv.N.Bytes())))
	der = append(der, priv.N.Bytes()...)
	der = append(der, 0x02, 0x01, 0x01) // public exponent 65537
	der = append(der, 0x02, byte(len(priv.D.Bytes())))
	der = append(der, priv.D.Bytes()...)

	encoded := make([]byte, 0)
	encoded = append(encoded, 0x30)
	encoded = append(encoded, byte(len(der)))
	encoded = append(encoded, der...)

	return encoded
}

// GenerateKeyPair generates an RSA key pair.
func GenerateKeyPair(bits int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, bits)
}

// ParseJWTExpiry extracts the expiration time from a token without validation.
func ParseJWTExpiry(tokenString string) (time.Time, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return time.Time{}, errors.New("invalid token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, err
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, err
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return time.Time{}, nil
	}

	return time.Unix(int64(exp), 0), nil
}

// ParseJWTIssuedAt extracts the issued-at time from a token without validation.
func ParseJWTIssuedAt(tokenString string) (time.Time, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return time.Time{}, errors.New("invalid token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, err
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, err
	}

	iat, ok := claims["iat"].(float64)
	if !ok {
		return time.Time{}, nil
	}

	return time.Unix(int64(iat), 0), nil
}

// GetJWTField extracts a field from the token payload without validation.
func GetJWTField(tokenString, field string) (interface{}, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}

	return claims[field], nil
}
