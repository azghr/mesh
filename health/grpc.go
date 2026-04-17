// Package health provides health checking and monitoring utilities.
//
// This package includes health checkers, probes for Kubernetes liveness,
// readiness, and startup checks, and deep checks for infrastructure dependencies.
//
// # Overview
//
// The health package provides:
//
//   - DeepHealthChecker: Aggregates multiple probes by check type
//   - CheckType: liveness, readiness, and startup probe types
//   - Built-in checks: Database, Redis, gRPC, and composite checks
//   - ReadinessResponse: JSON response format for /health endpoint
//
// This implementation follows Kubernetes probe patterns and the standard
// gRPC Health Checking Protocol.
//
// # Probe Types
//
// Kubernetes supports three probe types:
//
//   - Liveness: Is the container alive? (restart if fail)
//   - Readiness: Is the container ready to serve traffic?
//   - Startup: Has the container started? (disable others until pass)
package health

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	grpchealth "google.golang.org/grpc/health/grpc_health_v1"
)

const (
	GRPCHealthService = "grpc.health.v1.Health"
)

type GrpcHealthConfig struct {
	Address string
	Timeout time.Duration
}

func GRPCReadinessCheck(address string, cfg GrpcHealthConfig) func(ctx context.Context) error {
	if cfg.Timeout == 0 {
		cfg.Timeout = 3 * time.Second
	}

	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()

		conn, err := grpc.DialContext(ctx, address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			return fmt.Errorf("failed to connect to gRPC service: %w", err)
		}
		defer conn.Close()

		client := grpchealth.NewHealthClient(conn)
		resp, err := client.Check(ctx, &grpchealth.HealthCheckRequest{})
		if err != nil {
			return fmt.Errorf("gRPC health check failed: %w", err)
		}

		if resp.GetStatus() != grpchealth.HealthCheckResponse_SERVING {
			return fmt.Errorf("gRPC service not serving: %v", resp.GetStatus())
		}

		return nil
	}
}

func GRPCLivenessCheck(address string, cfg GrpcHealthConfig) func(ctx context.Context) error {
	if cfg.Timeout == 0 {
		cfg.Timeout = 3 * time.Second
	}

	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()

		conn, err := grpc.DialContext(ctx, address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			return fmt.Errorf("failed to connect to gRPC service: %w", err)
		}
		defer conn.Close()

		client := grpchealth.NewHealthClient(conn)
		_, err = client.Check(ctx, &grpchealth.HealthCheckRequest{})
		return err
	}
}

type GRPCHealthServer struct {
	mu       sync.Mutex
	services map[string]grpchealth.HealthCheckResponse_ServingStatus
	status   grpchealth.HealthCheckResponse_ServingStatus
}

func NewGRPCHealthServer() *GRPCHealthServer {
	return &GRPCHealthServer{
		services: make(map[string]grpchealth.HealthCheckResponse_ServingStatus),
		status:   grpchealth.HealthCheckResponse_SERVING,
	}
}

func (s *GRPCHealthServer) SetServingStatus(status grpchealth.HealthCheckResponse_ServingStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

func (s *GRPCHealthServer) SetServiceStatus(service string, status grpchealth.HealthCheckResponse_ServingStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services[service] = status
}

func (s *GRPCHealthServer) GetServingStatus(service string) grpchealth.HealthCheckResponse_ServingStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	if service == "" {
		return s.status
	}

	if status, ok := s.services[service]; ok {
		return status
	}
	return grpchealth.HealthCheckResponse_SERVICE_UNKNOWN
}

func (s *GRPCHealthServer) Check(ctx context.Context, req *grpchealth.HealthCheckRequest) (*grpchealth.HealthCheckResponse, error) {
	return &grpchealth.HealthCheckResponse{
		Status: s.GetServingStatus(req.Service),
	}, nil
}

func (s *GRPCHealthServer) Watch(req *grpchealth.HealthCheckRequest, stream grpchealth.Health_WatchServer) error {
	return nil
}

func (s *GRPCHealthServer) GRPCheck() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		status := s.GetServingStatus("")
		if status != grpchealth.HealthCheckResponse_SERVING {
			return fmt.Errorf("gRPC health not serving: %v", status)
		}
		return nil
	}
}
