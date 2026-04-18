# mesh

Distributed service mesh for service discovery, load balancing, and circuit breaking.

## What It Does

Provides a service mesh layer for managing microservices:
- Service registration and discovery
- Load balancing across instances
- Circuit breaker per service
- Health tracking

## Usage

### Create Service Mesh

```go
mesh := mesh.NewServiceMesh()
```

### Register Instances

```go
mesh.RegisterInstance(mesh.ServiceInstance{
    ServiceName: "user-service",
    Host:       "10.0.0.1",
    Port:       8080,
})
```

### Discover Services

```go
instances, err := mesh.Discover("user-service")
// instances := []ServiceInstance{{Host: "10.0.0.1", Port: 8080, Healthy: true}}
```

### Load Balancing

```go
instance := mesh.GetLoadBalancer(mesh.RoundRobin).Next("user-service")
```

### Circuit Breaker

```go
cb := mesh.GetCircuitBreaker("user-service")
cb.RecordSuccess()
cb.RecordFailure()
state := cb.GetState()
```

## Configuration

### Logger

```go
mesh := mesh.NewServiceMesh(mesh.WithLogger(logger))
```

### Health Check Interval

```go
mesh := mesh.NewServiceMesh(mesh.WithHealthCheckInterval(30 * time.Second))
```

## Load Balancing Strategies

- **RoundRobin**: Iterates through instances sequentially
- **Random**: Randomly selects an instance
- **LeastConnections**: Selects instance with fewest active connections

## Circuit Breaker States

- **CLOSED**: Normal operation
- **OPEN**: Too many failures, requests blocked
- **HALF_OPEN**: Testing recovery