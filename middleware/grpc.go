// Package middleware provides gRPC interceptors for logging, recovery, rate limiting, and circuit breaking.
//
// # Server Interceptors
//
//	server := grpc.NewServer(
//	    grpc.ChainUnaryInterceptor(
//	        middleware.GRPCLogging(),
//	        middleware.GRPCRecovery(),
//	        middleware.GRPCCircuitBreaker(),
//	        middleware.GRPCRateLimit(limiter),
//	    ),
//	)
//
// # Client Interceptors
//
//	conn, err := grpc.Dial(
//	    "localhost:50051",
//	    grpc.WithUnaryInterceptor(middleware.GRPCClientLogging()),
//	)
package middleware

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/azghr/mesh/logger"
	"github.com/azghr/mesh/ratelimiter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type (
	GRPCLoggingConfig struct {
		logger             logger.Logger
		logRequests        bool
		logResponses       bool
		logDuration        bool
		interestingMethods []string
	}

	GRPCLogOption func(*GRPCLoggingConfig)

	GRPCRecoveryConfig struct {
		logger          logger.Logger
		recoveryHandler func(p any) (any, error)
	}

	GRPCRecoveryOption func(*GRPCRecoveryConfig)

	GRPCRateLimitConfig struct {
		limiter    ratelimiter.RateLimiter
		keyPrefix  string
		byMethod   bool
		byUserID   bool
		userIDFunc func(ctx context.Context) string
	}

	GRPCRateLimitOption func(*GRPCRateLimitConfig)

	GRPCCircuitBreakerConfig struct {
		maxFailures  int
		resetTimeout time.Duration
		byMethod     bool
		breakers     map[string]*grpcMethodCircuitBreaker
		mu           sync.RWMutex
	}

	GRPCCircuitBreakerOption func(*GRPCCircuitBreakerConfig)

	grpcMethodCircuitBreaker struct {
		failures    int
		lastFailure time.Time
		state       GRPCCircuitState
	}

	GRPCCircuitState int
)

const (
	GRPCCircuitClosed GRPCCircuitState = iota
	GRPCCircuitOpen
	GRPCCircuitHalfOpen
)

func WithGRPCLogger(l logger.Logger) GRPCLogOption {
	return func(c *GRPCLoggingConfig) {
		c.logger = l
	}
}

func WithGRPCLogRequests(enabled bool) GRPCLogOption {
	return func(c *GRPCLoggingConfig) {
		c.logRequests = enabled
	}
}

func WithGRPCLogResponses(enabled bool) GRPCLogOption {
	return func(c *GRPCLoggingConfig) {
		c.logResponses = enabled
	}
}

func WithGRPCLogDuration(enabled bool) GRPCLogOption {
	return func(c *GRPCLoggingConfig) {
		c.logDuration = enabled
	}
}

func WithGRPCInterestingMethods(methods ...string) GRPCLogOption {
	return func(c *GRPCLoggingConfig) {
		c.interestingMethods = methods
	}
}

func grpcShouldLog(method string, interesting []string) bool {
	if interesting == nil {
		return true
	}
	for _, m := range interesting {
		if method == m {
			return true
		}
	}
	return false
}

func WithGRPCRecoveryLogger(l logger.Logger) GRPCRecoveryOption {
	return func(c *GRPCRecoveryConfig) {
		c.logger = l
	}
}

func WithGRPCRecoveryHandler(fn func(p any) (any, error)) GRPCRecoveryOption {
	return func(c *GRPCRecoveryConfig) {
		c.recoveryHandler = fn
	}
}

func grpcDefaultRecoveryHandler(p any) (any, error) {
	return nil, status.Errorf(codes.Internal, "panic recovered: %v", p)
}

func WithGRPCRateLimiter(limiter ratelimiter.RateLimiter) GRPCRateLimitOption {
	return func(c *GRPCRateLimitConfig) {
		c.limiter = limiter
	}
}

func WithGRPCRateLimitKeyPrefix(prefix string) GRPCRateLimitOption {
	return func(c *GRPCRateLimitConfig) {
		c.keyPrefix = prefix
	}
}

func WithGRPCRateLimitByMethod() GRPCRateLimitOption {
	return func(c *GRPCRateLimitConfig) {
		c.byMethod = true
	}
}

func WithGRPCRateLimitByUserID(fn func(ctx context.Context) string) GRPCRateLimitOption {
	return func(c *GRPCRateLimitConfig) {
		c.byUserID = true
		c.userIDFunc = fn
	}
}

func WithGRPCCircuitBreakerMaxFailures(n int) GRPCCircuitBreakerOption {
	return func(c *GRPCCircuitBreakerConfig) {
		c.maxFailures = n
	}
}

func WithGRPCCircuitBreakerResetTimeout(d time.Duration) GRPCCircuitBreakerOption {
	return func(c *GRPCCircuitBreakerConfig) {
		c.resetTimeout = d
	}
}

func WithGRPCCircuitBreakerByMethod() GRPCCircuitBreakerOption {
	return func(c *GRPCCircuitBreakerConfig) {
		c.byMethod = true
	}
}

func GRPCLogging(opts ...GRPCLogOption) grpc.UnaryServerInterceptor {
	cfg := &GRPCLoggingConfig{
		logger:             logger.GetGlobal(),
		logRequests:        false,
		logResponses:       false,
		logDuration:        true,
		interestingMethods: nil,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		var requestID, userID string
		if ok {
			if vals := md.Get("x-request-id"); len(vals) > 0 {
				requestID = vals[0]
			}
			if vals := md.Get("x-user-id"); len(vals) > 0 {
				userID = vals[0]
			}
		}

		fullMethod := info.FullMethod
		start := time.Now()

		if requestID == "" {
			requestID = fmt.Sprintf("%d-%d", time.Now().Unix(), time.Now().Nanosecond())
		}

		logCtx := map[string]any{
			"request_id": requestID,
			"method":     fullMethod,
			"user_id":    userID,
		}

		if cfg.logRequests && grpcShouldLog(fullMethod, cfg.interestingMethods) {
			cfg.logger.Info("gRPC request started", logCtx)
		}

		resp, err := handler(ctx, req)
		duration := time.Since(start)

		if cfg.logDuration || (err != nil && grpcShouldLog(fullMethod, cfg.interestingMethods)) {
			logCtx["duration_ms"] = duration.Milliseconds()
			if err != nil {
				st, _ := status.FromError(err)
				logCtx["code"] = st.Code().String()
				cfg.logger.Error("gRPC request failed", logCtx)
			} else if grpcShouldLog(fullMethod, cfg.interestingMethods) {
				cfg.logger.Info("gRPC request completed", logCtx)
			}
		}

		return resp, err
	}
}

func GRPCRecovery(opts ...GRPCRecoveryOption) grpc.UnaryServerInterceptor {
	cfg := &GRPCRecoveryConfig{
		logger:          logger.GetGlobal(),
		recoveryHandler: grpcDefaultRecoveryHandler,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		defer func() {
			if r := recover(); r != nil {
				cfg.logger.Error("panic recovered",
					map[string]any{
						"panic": fmt.Sprintf("%v", r),
					})
				_, _ = cfg.recoveryHandler(r)
			}
		}()
		return handler(ctx, req)
	}
}

func GRPCRateLimit(limiter ratelimiter.RateLimiter, opts ...GRPCRateLimitOption) grpc.UnaryServerInterceptor {
	cfg := &GRPCRateLimitConfig{
		limiter:   limiter,
		keyPrefix: "grpc",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if cfg.limiter == nil {
			return handler(ctx, req)
		}

		fullMethod := info.FullMethod
		key := cfg.keyPrefix + ":" + fullMethod
		if cfg.byUserID && cfg.userIDFunc != nil {
			if userID := cfg.userIDFunc(ctx); userID != "" {
				key += ":" + userID
			}
		} else if cfg.byMethod {
			key = cfg.keyPrefix + ":" + fullMethod
		}

		allowed, err := cfg.limiter.Allow(ctx, key)
		if err != nil {
			return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded: %v", err)
		}
		if !allowed {
			return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded for %s", fullMethod)
		}

		return handler(ctx, req)
	}
}

func GRPCCircuitBreaker(opts ...GRPCCircuitBreakerOption) grpc.UnaryServerInterceptor {
	cfg := &GRPCCircuitBreakerConfig{
		maxFailures:  5,
		resetTimeout: 30 * time.Second,
		byMethod:     true,
		breakers:     make(map[string]*grpcMethodCircuitBreaker),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		key := info.FullMethod
		if !cfg.byMethod {
			key = "*"
		}

		cfg.mu.RLock()
		cb, exists := cfg.breakers[key]
		cfg.mu.RUnlock()

		if !exists {
			cfg.mu.Lock()
			if cb, exists = cfg.breakers[key]; !exists {
				cb = &grpcMethodCircuitBreaker{state: GRPCCircuitClosed}
				cfg.breakers[key] = cb
			}
			cfg.mu.Unlock()
		}

		cfg.mu.Lock()
		defer cfg.mu.Unlock()

		switch cb.state {
		case GRPCCircuitOpen:
			if time.Since(cb.lastFailure) > cfg.resetTimeout {
				cb.state = GRPCCircuitHalfOpen
				return handler(ctx, req)
			}
			return nil, status.Errorf(codes.Unavailable, "circuit breaker open for %s", key)

		case GRPCCircuitHalfOpen:
			fallthrough

		case GRPCCircuitClosed:
			resp, err := handler(ctx, req)
			if err != nil {
				cb.failures++
				cb.lastFailure = time.Now()
				if cb.failures >= cfg.maxFailures {
					cb.state = GRPCCircuitOpen
				}
			} else {
				cb.failures = 0
				if cb.state == GRPCCircuitHalfOpen {
					cb.state = GRPCCircuitClosed
				}
			}
			return resp, err
		}

		return handler(ctx, req)
	}
}

func GRPCLoggingStream(opts ...GRPCLogOption) grpc.StreamServerInterceptor {
	cfg := &GRPCLoggingConfig{
		logger:             logger.GetGlobal(),
		logRequests:        true,
		logResponses:       false,
		logDuration:        true,
		interestingMethods: nil,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		md, _ := metadata.FromIncomingContext(ctx)
		requestID := fmt.Sprintf("%d-%d", time.Now().Unix(), time.Now().Nanosecond())
		if vals := md.Get("x-request-id"); len(vals) > 0 {
			requestID = vals[0]
		}

		start := time.Now()
		err := handler(srv, ss)
		duration := time.Since(start)

		logCtx := map[string]any{
			"request_id":  requestID,
			"method":      info.FullMethod,
			"duration_ms": duration.Milliseconds(),
		}

		l := logger.GetGlobal()
		if err != nil {
			st, _ := status.FromError(err)
			logCtx["code"] = st.Code().String()
			l.Error("gRPC stream failed", logCtx)
		} else {
			l.Info("gRPC stream completed", logCtx)
		}

		return err
	}
}

func GRPCRecoveryStream(opts ...GRPCRecoveryOption) grpc.StreamServerInterceptor {
	cfg := &GRPCRecoveryConfig{
		logger:          logger.GetGlobal(),
		recoveryHandler: grpcDefaultRecoveryHandler,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		defer func() {
			if r := recover(); r != nil {
				cfg.logger.Error("panic recovered in stream",
					map[string]any{
						"panic": fmt.Sprintf("%v", r),
					})
				_, _ = cfg.recoveryHandler(r)
			}
		}()
		return handler(srv, ss)
	}
}

func GRPCRateLimitStream(limiter ratelimiter.RateLimiter, opts ...GRPCRateLimitOption) grpc.StreamServerInterceptor {
	cfg := &GRPCRateLimitConfig{
		limiter:   limiter,
		keyPrefix: "grpc",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if cfg.limiter == nil {
			return handler(srv, ss)
		}

		ctx := ss.Context()
		fullMethod := info.FullMethod
		key := cfg.keyPrefix + ":" + fullMethod

		allowed, err := cfg.limiter.Allow(ctx, key)
		if err != nil {
			return status.Errorf(codes.ResourceExhausted, "rate limit exceeded: %v", err)
		}
		if !allowed {
			return status.Errorf(codes.ResourceExhausted, "rate limit exceeded for %s", fullMethod)
		}

		return handler(srv, ss)
	}
}

func GRPCClientLogging(opts ...GRPCLogOption) func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, callOpts ...grpc.CallOption) error {
	cfg := &GRPCLoggingConfig{
		logger:             logger.GetGlobal(),
		logRequests:        false,
		logResponses:       false,
		logDuration:        true,
		interestingMethods: nil,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, callOpts ...grpc.CallOption) error {
		md, _ := metadata.FromOutgoingContext(ctx)
		requestID := fmt.Sprintf("%d-%d", time.Now().Unix(), time.Now().Nanosecond())
		if vals := md.Get("x-request-id"); len(vals) > 0 {
			requestID = vals[0]
		}

		start := time.Now()

		logCtx := map[string]any{
			"request_id": requestID,
			"method":     method,
		}

		if cfg.logRequests && grpcShouldLog(method, cfg.interestingMethods) {
			cfg.logger.Info("gRPC client request started", logCtx)
		}

		err := invoker(ctx, method, req, reply, cc, callOpts...)
		duration := time.Since(start)

		if cfg.logDuration || (err != nil && grpcShouldLog(method, cfg.interestingMethods)) {
			logCtx["duration_ms"] = duration.Milliseconds()
			if err != nil {
				st, _ := status.FromError(err)
				logCtx["code"] = st.Code().String()
				cfg.logger.Error("gRPC client request failed", logCtx)
			} else if grpcShouldLog(method, cfg.interestingMethods) {
				cfg.logger.Info("gRPC client request completed", logCtx)
			}
		}

		return err
	}
}

func GRPCClientRateLimit(limiter ratelimiter.RateLimiter, opts ...GRPCRateLimitOption) func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, callOpts ...grpc.CallOption) error {
	cfg := &GRPCRateLimitConfig{
		limiter:   limiter,
		keyPrefix: "grpc",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, callOpts ...grpc.CallOption) error {
		if cfg.limiter == nil {
			return invoker(ctx, method, req, reply, cc, callOpts...)
		}

		key := cfg.keyPrefix + ":" + method
		if cfg.byUserID && cfg.userIDFunc != nil {
			if userID := cfg.userIDFunc(ctx); userID != "" {
				key += ":" + userID
			}
		} else if cfg.byMethod {
			key = cfg.keyPrefix + ":" + method
		}

		allowed, err := cfg.limiter.Allow(ctx, key)
		if err != nil {
			return status.Errorf(codes.ResourceExhausted, "rate limit exceeded: %v", err)
		}
		if !allowed {
			return status.Errorf(codes.ResourceExhausted, "rate limit exceeded for %s", method)
		}

		return invoker(ctx, method, req, reply, cc, callOpts...)
	}
}

func GRPCClientLoggingStream(opts ...GRPCLogOption) func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, callOpts ...grpc.CallOption) (grpc.ClientStream, error) {
	cfg := &GRPCLoggingConfig{
		logger:             logger.GetGlobal(),
		logRequests:        true,
		logResponses:       false,
		logDuration:        true,
		interestingMethods: nil,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, callOpts ...grpc.CallOption) (grpc.ClientStream, error) {
		md, _ := metadata.FromOutgoingContext(ctx)
		requestID := fmt.Sprintf("%d-%d", time.Now().Unix(), time.Now().Nanosecond())
		if vals := md.Get("x-request-id"); len(vals) > 0 {
			requestID = vals[0]
		}

		logCtx := map[string]any{
			"request_id": requestID,
			"method":     method,
		}

		stream, err := streamer(ctx, desc, cc, method, callOpts...)

		l := logger.GetGlobal()
		if err != nil {
			st, _ := status.FromError(err)
			logCtx["code"] = st.Code().String()
			l.Error("gRPC client stream failed", logCtx)
		} else if cfg.logRequests {
			l.Info("gRPC client stream started", logCtx)
		}

		return stream, err
	}
}

func GRPCClientRateLimitStream(limiter ratelimiter.RateLimiter, opts ...GRPCRateLimitOption) func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, callOpts ...grpc.CallOption) (grpc.ClientStream, error) {
	cfg := &GRPCRateLimitConfig{
		limiter:   limiter,
		keyPrefix: "grpc",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, callOpts ...grpc.CallOption) (grpc.ClientStream, error) {
		if cfg.limiter == nil {
			return streamer(ctx, desc, cc, method, callOpts...)
		}

		key := cfg.keyPrefix + ":" + method

		allowed, err := cfg.limiter.Allow(ctx, key)
		if err != nil {
			return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded: %v", err)
		}
		if !allowed {
			return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded for %s", method)
		}

		return streamer(ctx, desc, cc, method, callOpts...)
	}
}

func GRPCGetRequestID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if vals := md.Get("x-request-id"); len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func GRPCGetUserID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if vals := md.Get("x-user-id"); len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func GRPCWithRequestID(ctx context.Context, requestID string) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(map[string]string{})
	} else {
		md = md.Copy()
	}
	md.Set("x-request-id", requestID)
	return metadata.NewOutgoingContext(ctx, md)
}

func GRPCWithUserID(ctx context.Context, userID string) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(map[string]string{})
	} else {
		md = md.Copy()
	}
	md.Set("x-user-id", userID)
	return metadata.NewOutgoingContext(ctx, md)
}

func GRPCGetTraceID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if vals := md.Get("x-trace-id"); len(vals) > 0 {
		return vals[0]
	}
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(time.Now().UnixNano()))
	return fmt.Sprintf("%x", b)
}

func GRPCWithTraceID(ctx context.Context, traceID string) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(map[string]string{})
	} else {
		md = md.Copy()
	}
	md.Set("x-trace-id", traceID)
	return metadata.NewOutgoingContext(ctx, md)
}

func GRPCChainUnaryInterceptor(
	logging grpc.UnaryServerInterceptor,
	recovery grpc.UnaryServerInterceptor,
	rateLimit grpc.UnaryServerInterceptor,
	circuitBreaker grpc.UnaryServerInterceptor,
) []grpc.UnaryServerInterceptor {
	interceptors := []grpc.UnaryServerInterceptor{}
	if logging != nil {
		interceptors = append(interceptors, logging)
	}
	if recovery != nil {
		interceptors = append(interceptors, recovery)
	}
	if rateLimit != nil {
		interceptors = append(interceptors, rateLimit)
	}
	if circuitBreaker != nil {
		interceptors = append(interceptors, circuitBreaker)
	}
	return interceptors
}

func GRPCChainStreamInterceptor(
	logging grpc.StreamServerInterceptor,
	recovery grpc.StreamServerInterceptor,
	rateLimit grpc.StreamServerInterceptor,
) []grpc.StreamServerInterceptor {
	interceptors := []grpc.StreamServerInterceptor{}
	if logging != nil {
		interceptors = append(interceptors, logging)
	}
	if recovery != nil {
		interceptors = append(interceptors, recovery)
	}
	if rateLimit != nil {
		interceptors = append(interceptors, rateLimit)
	}
	return interceptors
}

func GRPCChainUnaryClientInterceptor(
	logging grpc.UnaryClientInterceptor,
	rateLimit grpc.UnaryClientInterceptor,
) []grpc.UnaryClientInterceptor {
	interceptors := []grpc.UnaryClientInterceptor{}
	if logging != nil {
		interceptors = append(interceptors, logging)
	}
	if rateLimit != nil {
		interceptors = append(interceptors, rateLimit)
	}
	return interceptors
}

func GRPCChainStreamClientInterceptor(
	logging grpc.StreamClientInterceptor,
	rateLimit grpc.StreamClientInterceptor,
) []grpc.StreamClientInterceptor {
	interceptors := []grpc.StreamClientInterceptor{}
	if logging != nil {
		interceptors = append(interceptors, logging)
	}
	if rateLimit != nil {
		interceptors = append(interceptors, rateLimit)
	}
	return interceptors
}

func timeSince(t time.Time) time.Duration {
	return time.Since(t)
}
