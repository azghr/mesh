// Package mesh provides service discovery and mesh integration.
//
// This package provides service mesh capabilities including:
//
//   - Service registry with health tracking
//   - Load balancing (round robin, random, least connections)
//   - Circuit breaker per service
//   - Service discovery
//
// Quick Example:
//
//	mesh := mesh.NewServiceMesh(mesh.ServiceMeshConfig{
//	    ServiceName: "user-service",
//	    ServiceVersion: "v1.2.0",
//	})
//
//	// Register instances
//	mesh.RegisterInstance(mesh.ServiceInstance{
//	    ServiceName: "user-service",
//	    Host:       "10.0.0.1",
//	    Port:       8080,
//	    Weight:     100,
//	})
//
//	// Get healthy instance
//	instance, err := mesh.Choose("user-service", mesh.RoundRobin())
//
//	// With circuit breaker
//	err := mesh.ExecuteWithCircuit("user-service", func() error {
//	    return callService(instance)
//	})
package mesh

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/azghr/mesh/logger"
)

type (
	ServiceMeshConfig struct {
		ServiceName    string
		ServiceVersion string
		Logger         logger.Logger
	}

	ServiceInstance struct {
		ServiceName string
		Host        string
		Port        int
		Weight      int
		Healthy     bool
		LastCheck   time.Time
		Metadata    map[string]string
	}

	ServiceRegistry struct {
		services map[string]*Service
		mu       sync.RWMutex
	}

	Service struct {
		Name      string
		Instances []ServiceInstance
		CB        *CircuitBreakerState
	}

	CircuitBreakerState struct {
		failures    int
		lastFailure time.Time
		state       CircuitState
		mu          sync.Mutex
	}

	CircuitState int

	LoadBalancer interface {
		Choose(*Service) *ServiceInstance
	}

	ServiceMesh struct {
		config   *ServiceMeshConfig
		registry *ServiceRegistry
		logger   logger.Logger
	}

	ServiceMeshOption func(*ServiceMeshConfig)
)

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

func WithServiceName(name string) ServiceMeshOption {
	return func(c *ServiceMeshConfig) {
		c.ServiceName = name
	}
}

func WithServiceVersion(version string) ServiceMeshOption {
	return func(c *ServiceMeshConfig) {
		c.ServiceVersion = version
	}
}

func WithMeshLogger(l logger.Logger) ServiceMeshOption {
	return func(c *ServiceMeshConfig) {
		c.Logger = l
	}
}

func NewServiceMesh(opts ...ServiceMeshOption) *ServiceMesh {
	cfg := &ServiceMeshConfig{
		Logger: logger.GetGlobal(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return &ServiceMesh{
		config:   cfg,
		registry: &ServiceRegistry{services: make(map[string]*Service)},
		logger:   cfg.Logger,
	}
}

func (s *ServiceMesh) RegisterInstance(instance ServiceInstance) error {
	s.registry.mu.Lock()
	defer s.registry.mu.Unlock()

	instance.LastCheck = time.Now()

	service, exists := s.registry.services[instance.ServiceName]
	if !exists {
		service = &Service{
			Name:      instance.ServiceName,
			Instances: []ServiceInstance{},
			CB: &CircuitBreakerState{
				state: StateClosed,
			},
		}
		s.registry.services[instance.ServiceName] = service
	}

	service.Instances = append(service.Instances, instance)

	s.logger.Info("instance registered",
		"service", instance.ServiceName,
		"host", instance.Host,
		"port", instance.Port,
	)

	return nil
}

func (s *ServiceMesh) Discover(serviceName string) ([]ServiceInstance, error) {
	s.registry.mu.RLock()
	defer s.registry.mu.RUnlock()

	service, exists := s.registry.services[serviceName]
	if !exists {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}

	var healthy []ServiceInstance
	for _, inst := range service.Instances {
		if inst.Healthy {
			healthy = append(healthy, inst)
		}
	}

	return healthy, nil
}

func (s *ServiceMesh) Choose(serviceName string, lb LoadBalancer) (*ServiceInstance, error) {
	s.registry.mu.RLock()
	defer s.registry.mu.RUnlock()

	service, exists := s.registry.services[serviceName]
	if !exists {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}

	if len(service.Instances) == 0 {
		return nil, fmt.Errorf("no instances for service %s", serviceName)
	}

	instance := lb.Choose(service)
	if instance == nil {
		return nil, fmt.Errorf("no healthy instance for service %s", serviceName)
	}

	return instance, nil
}

type RoundRobinBalancer struct {
	current int
	mu      sync.Mutex
}

func (r *RoundRobinBalancer) Choose(svc *Service) *ServiceInstance {
	r.mu.Lock()
	defer r.mu.Unlock()

	if svc == nil || len(svc.Instances) == 0 {
		return nil
	}

	instance := svc.Instances[r.current%len(svc.Instances)]
	r.current++

	return &instance
}

func RoundRobin() *RoundRobinBalancer {
	return &RoundRobinBalancer{}
}

type RandomBalancer struct{}

func (r *RandomBalancer) Choose(svc *Service) *ServiceInstance {
	if svc == nil || len(svc.Instances) == 0 {
		return nil
	}

	idx := rand.Intn(len(svc.Instances))
	return &svc.Instances[idx]
}

func Random() *RandomBalancer {
	return &RandomBalancer{}
}

type LeastConnectionsBalancer struct{}

func (l *LeastConnectionsBalancer) Choose(svc *Service) *ServiceInstance {
	if svc == nil || len(svc.Instances) == 0 {
		return nil
	}

	minVal := 0
	best := &svc.Instances[0]

	for i := range svc.Instances {
		conns := svc.Instances[i].Metadata["connections"]
		var val int
		fmt.Sscanf(conns, "%d", &val)
		if val < minVal || i == 0 {
			minVal = val
			best = &svc.Instances[i]
		}
	}

	return best
}

func LeastConnections() *LeastConnectionsBalancer {
	return &LeastConnectionsBalancer{}
}

func (s *ServiceMesh) GetCircuitBreaker(serviceName string) *CircuitBreakerState {
	s.registry.mu.RLock()
	defer s.registry.mu.RUnlock()

	service, exists := s.registry.services[serviceName]
	if !exists {
		return nil
	}

	return service.CB
}

func (s *ServiceMesh) ExecuteWithCircuit(serviceName string, fn func() error) error {
	s.registry.mu.RLock()
	service, exists := s.registry.services[serviceName]
	s.registry.mu.RUnlock()

	if !exists {
		return fn()
	}

	cb := service.CB
	if cb == nil {
		return fn()
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateOpen:
		if time.Since(cb.lastFailure) > 30*time.Second {
			cb.state = StateHalfOpen
			return fn()
		}
		return fmt.Errorf("circuit breaker open for %s", serviceName)
	case StateHalfOpen:
		err := fn()
		if err != nil {
			cb.state = StateOpen
			cb.lastFailure = time.Now()
			cb.failures++
			return err
		}
		cb.state = StateClosed
		cb.failures = 0
		return nil
	default:
		err := fn()
		if err != nil {
			cb.failures++
			if cb.failures >= 5 {
				cb.state = StateOpen
			}
			cb.lastFailure = time.Now()
		} else {
			cb.failures = 0
		}
		return err
	}
}

func (s *ServiceMesh) Heartbeat(serviceName, host string, port int) error {
	s.registry.mu.Lock()
	defer s.registry.mu.Unlock()

	service, exists := s.registry.services[serviceName]
	if !exists {
		return fmt.Errorf("service %s not found", serviceName)
	}

	for i := range service.Instances {
		if service.Instances[i].Host == host && service.Instances[i].Port == port {
			service.Instances[i].Healthy = true
			service.Instances[i].LastCheck = time.Now()
			return nil
		}
	}

	return fmt.Errorf("instance not found")
}

func (s *ServiceMesh) DeregisterInstance(serviceName string, host string, port int) error {
	s.registry.mu.Lock()
	defer s.registry.mu.Unlock()

	service, exists := s.registry.services[serviceName]
	if !exists {
		return fmt.Errorf("service %s not found", serviceName)
	}

	for i := range service.Instances {
		if service.Instances[i].Host == host && service.Instances[i].Port == port {
			service.Instances = append(service.Instances[:i], service.Instances[i+1:]...)
			s.logger.Info("instance deregistered",
				"service", serviceName,
				"host", host,
				"port", port,
			)
			return nil
		}
	}

	return fmt.Errorf("instance not found")
}

func (s *ServiceMesh) HealthCount(serviceName string) (healthy, total int) {
	s.registry.mu.RLock()
	defer s.registry.mu.RUnlock()

	service, exists := s.registry.services[serviceName]
	if !exists {
		return 0, 0
	}

	total = len(service.Instances)
	for _, inst := range service.Instances {
		if inst.Healthy {
			healthy++
		}
	}

	return healthy, total
}

func (i *ServiceInstance) Address() string {
	return fmt.Sprintf("%s:%d", i.Host, i.Port)
}
