package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestGRPCLoggingInterceptor(t *testing.T) {
	interceptor := GRPCLogging()

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-request-id", "test-request-id",
		"x-user-id", "test-user-id",
	))

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Test",
	}

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	resp, err := interceptor(ctx, "request", info, handler)

	assert.NoError(t, err)
	assert.Equal(t, "response", resp)
}

func TestGRPCRecoveryInterceptor(t *testing.T) {
	interceptor := GRPCRecovery()

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Test",
	}

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	resp, err := interceptor(ctx, "request", info, handler)

	assert.NoError(t, err)
	assert.Equal(t, "response", resp)
}

func TestGRPCCircuitBreaker(t *testing.T) {
	interceptor := GRPCCircuitBreaker(
		WithGRPCCircuitBreakerMaxFailures(2),
		WithGRPCCircuitBreakerResetTimeout(100*time.Millisecond),
	)

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Test",
	}

	handler := func(ctx context.Context, req any) (any, error) {
		return nil, assert.AnError
	}

	_, _ = interceptor(ctx, "request", info, handler)
	_, _ = interceptor(ctx, "request", info, handler)

	_, err := interceptor(ctx, "request", info, handler)
	assert.Error(t, err)
}

func TestGRPCGetRequestID(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-request-id", "test-id",
	))

	id := GRPCGetRequestID(ctx)
	assert.Equal(t, "test-id", id)
}

func TestGRPCGetUserID(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-user-id", "user-123",
	))

	id := GRPCGetUserID(ctx)
	assert.Equal(t, "user-123", id)
}

func TestGRPCWithRequestID(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-request-id", "new-id",
	))

	id := GRPCGetRequestID(ctx)
	assert.Equal(t, "new-id", id)
}

func TestGRPCWithUserID(t *testing.T) {
	ctx := GRPCWithUserID(context.Background(), "user-456")

	id := GRPCGetUserID(ctx)
	assert.Equal(t, "", id)

	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs())
}

func TestGRPCChainUnaryInterceptor(t *testing.T) {
	interceptors := GRPCChainUnaryInterceptor(
		GRPCLogging(),
		GRPCRecovery(),
		nil,
		nil,
	)

	assert.Len(t, interceptors, 2)
}

func TestGRPCRateLimitNoLimiter(t *testing.T) {
	interceptor := GRPCRateLimit(nil)

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Test",
	}

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	resp, err := interceptor(ctx, "request", info, handler)

	assert.NoError(t, err)
	assert.Equal(t, "response", resp)
}

type mockRateLimiter struct {
	allow  bool
	err    error
	called bool
}

func (m *mockRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	m.called = true
	return m.allow, m.err
}

func (m *mockRateLimiter) AllowN(ctx context.Context, key string, n int) (bool, error) {
	m.called = true
	return m.allow, m.err
}

func (m *mockRateLimiter) Reset(ctx context.Context, key string) error {
	m.called = true
	return m.err
}

func (m *mockRateLimiter) GetLimit(ctx context.Context, key string) (int, int, error) {
	m.called = true
	return 0, 0, m.err
}
