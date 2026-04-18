package mesh

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewServiceMesh(t *testing.T) {
	mesh := NewServiceMesh()
	assert.NotNil(t, mesh)
}

func TestRegisterInstance(t *testing.T) {
	mesh := NewServiceMesh()

	err := mesh.RegisterInstance(ServiceInstance{
		ServiceName: "user-service",
		Host:        "10.0.0.1",
		Port:        8080,
		Weight:      100,
	})
	assert.NoError(t, err)
}

func TestDiscover(t *testing.T) {
	mesh := NewServiceMesh()

	mesh.RegisterInstance(ServiceInstance{
		ServiceName: "user-service",
		Host:        "10.0.0.1",
		Port:        8080,
		Weight:      100,
		Healthy:     true,
	})

	instances, err := mesh.Discover("user-service")
	assert.NoError(t, err)
	assert.Len(t, instances, 1)
	assert.Equal(t, "10.0.0.1", instances[0].Host)
}

func TestDiscoverNotFound(t *testing.T) {
	mesh := NewServiceMesh()

	_, err := mesh.Discover("unknown-service")
	assert.Error(t, err)
}

func TestChooseWithRoundRobin(t *testing.T) {
	mesh := NewServiceMesh()

	mesh.RegisterInstance(ServiceInstance{ServiceName: "test", Host: "10.0.0.1", Port: 8080})
	mesh.RegisterInstance(ServiceInstance{ServiceName: "test", Host: "10.0.0.2", Port: 8080})

	instance, err := mesh.Choose("test", RoundRobin())
	assert.NoError(t, err)
	assert.NotNil(t, instance)
}

func TestChooseWithRandom(t *testing.T) {
	mesh := NewServiceMesh()

	mesh.RegisterInstance(ServiceInstance{ServiceName: "test", Host: "10.0.0.1", Port: 8080})

	instance, err := mesh.Choose("test", Random())
	assert.NoError(t, err)
	assert.NotNil(t, instance)
}

func TestHeartbeat(t *testing.T) {
	mesh := NewServiceMesh()

	mesh.RegisterInstance(ServiceInstance{
		ServiceName: "test",
		Host:        "10.0.0.1",
		Port:        8080,
		Healthy:     false,
	})

	err := mesh.Heartbeat("test", "10.0.0.1", 8080)
	assert.NoError(t, err)

	instances, _ := mesh.Discover("test")
	assert.True(t, instances[0].Healthy)
}

func TestDeregisterInstance(t *testing.T) {
	mesh := NewServiceMesh()

	mesh.RegisterInstance(ServiceInstance{
		ServiceName: "test",
		Host:        "10.0.0.1",
		Port:        8080,
	})

	err := mesh.DeregisterInstance("test", "10.0.0.1", 8080)
	assert.NoError(t, err)

	instances, _ := mesh.Discover("test")
	assert.Len(t, instances, 0)
}

func TestHealthCount(t *testing.T) {
	mesh := NewServiceMesh()

	mesh.RegisterInstance(ServiceInstance{ServiceName: "test", Host: "10.0.0.1", Port: 8080, Healthy: true})
	mesh.RegisterInstance(ServiceInstance{ServiceName: "test", Host: "10.0.0.2", Port: 8080, Healthy: false})

	healthy, total := mesh.HealthCount("test")
	assert.Equal(t, 1, healthy)
	assert.Equal(t, 2, total)
}

func TestServiceInstanceAddress(t *testing.T) {
	inst := &ServiceInstance{
		Host: "10.0.0.1",
		Port: 8080,
	}

	assert.Equal(t, "10.0.0.1:8080", inst.Address())
}

func TestGetCircuitBreaker(t *testing.T) {
	mesh := NewServiceMesh()

	mesh.RegisterInstance(ServiceInstance{ServiceName: "test", Host: "10.0.0.1", Port: 8080})

	cb := mesh.GetCircuitBreaker("test")
	assert.NotNil(t, cb)
}

func TestExecuteWithCircuitClosed(t *testing.T) {
	mesh := NewServiceMesh()

	mesh.RegisterInstance(ServiceInstance{ServiceName: "test", Host: "10.0.0.1", Port: 8080})

	err := mesh.ExecuteWithCircuit("test", func() error {
		return nil
	})
	assert.NoError(t, err)
}

func TestOptions(t *testing.T) {
	mesh := NewServiceMesh(
		WithServiceName("my-service"),
		WithServiceVersion("v1.0.0"),
	)
	assert.NotNil(t, mesh)
}
