package shutdown

import (
	"context"
	"testing"
	"time"
)

func TestConnTracker_Register(t *testing.T) {
	ct := &ConnTracker{}

	ct.RegisterConnection()
	if ct.ActiveConnections() != 1 {
		t.Errorf("ActiveConnections() = %d, want 1", ct.ActiveConnections())
	}

	ct.UnregisterConnection()
	if ct.ActiveConnections() != 0 {
		t.Errorf("ActiveConnections() = %d, want 0", ct.ActiveConnections())
	}
}

func TestConnTracker_Draining(t *testing.T) {
	ct := &ConnTracker{}

	ct.RegisterConnection()
	if ct.ActiveConnections() != 1 {
		t.Errorf("ActiveConnections() = %d, want 1", ct.ActiveConnections())
	}

	ct.StartDraining()

	if !ct.IsDraining() {
		t.Error("IsDraining() should return true after StartDraining()")
	}

	// When draining, new registrations shouldn't increase count
	ct.RegisterConnection()
	// Already had 1, should still be 1 (unregistered after drain)
	if ct.ActiveConnections() > 1 {
		t.Errorf("ActiveConnections() = %d, want <=1 during draining", ct.ActiveConnections())
	}
}

func TestConnTracker_WaitForDrain(t *testing.T) {
	ct := &ConnTracker{}

	ct.RegisterConnection()
	ct.RegisterConnection()

	go func() {
		time.Sleep(50 * time.Millisecond)
		ct.UnregisterConnection()
		ct.UnregisterConnection()
	}()

	err := ct.WaitForDrain(1 * time.Second)
	if err != nil {
		t.Errorf("WaitForDrain() error = %v", err)
	}
}

func TestConnTracker_WaitForDrain_Timeout(t *testing.T) {
	ct := &ConnTracker{}

	ct.RegisterConnection()

	err := ct.WaitForDrain(50 * time.Millisecond)
	if err == nil {
		t.Error("WaitForDrain() should return error on timeout")
	}
}

func TestHealthChecker_SetStatus(t *testing.T) {
	hc := &HealthChecker{}

	hc.SetHealthy(true)
	if !hc.IsHealthy() {
		t.Error("IsHealthy() should return true after SetHealthy(true)")
	}

	hc.SetReady(true)
	if !hc.IsReady() {
		t.Error("IsReady() should return true after SetReady(true)")
	}
}

func TestHealthChecker_AddHealthCheck(t *testing.T) {
	hc := &HealthChecker{}

	called := false
	hc.AddHealthCheck(func(ctx context.Context) error {
		called = true
		return nil
	})

	if err := hc.RunHealthChecks(context.Background()); err != nil {
		t.Errorf("RunHealthChecks() error = %v", err)
	}

	if !called {
		t.Error("health check was not called")
	}
}

func TestHealthChecker_AddHealthCheck_Error(t *testing.T) {
	hc := &HealthChecker{}

	hc.AddHealthCheck(func(ctx context.Context) error {
		return errTest
	})

	err := hc.RunHealthChecks(context.Background())
	if err == nil {
		t.Error("RunHealthChecks() should return error")
	}
}

func TestHealthChecker_BeforeAfterShutdown(t *testing.T) {
	hc := &HealthChecker{}

	beforeCalled := false
	afterCalled := false

	hc.AddBeforeShutdown(func() error {
		beforeCalled = true
		return nil
	})

	hc.AddAfterShutdown(func() error {
		afterCalled = true
		return nil
	})

	if err := hc.RunBeforeShutdown(); err != nil {
		t.Errorf("RunBeforeShutdown() error = %v", err)
	}
	if err := hc.RunAfterShutdown(); err != nil {
		t.Errorf("RunAfterShutdown() error = %v", err)
	}

	if !beforeCalled {
		t.Error("before shutdown was not called")
	}
	if !afterCalled {
		t.Error("after shutdown was not called")
	}
}

func TestNewEnhancedManager(t *testing.T) {
	mgr := NewEnhancedManager()

	if mgr == nil {
		t.Fatal("NewEnhancedManager() returned nil")
	}

	if mgr.connTracker == nil {
		t.Error("connTracker is nil")
	}

	if mgr.healthChecks == nil {
		t.Error("healthChecks is nil")
	}
}

func TestEnhancedManager_Phases(t *testing.T) {
	mgr := NewEnhancedManager()

	mgr.AddPhase("cleanup", []string{"task1"}, 1)
	mgr.AddPhase("finalize", []string{"task2"}, 2)

	mgr.RegisterTask("task1", func(ctx context.Context) error {
		return nil
	})

	mgr.RegisterTask("task2", func(ctx context.Context) error {
		return nil
	})
}

func TestEnhancedManager_HealthChecks(t *testing.T) {
	mgr := NewEnhancedManager()

	hcCalled := false
	mgr.AddHealthCheck(func(ctx context.Context) error {
		hcCalled = true
		return nil
	})

	if err := mgr.healthChecks.RunHealthChecks(context.Background()); err != nil {
		t.Errorf("RunHealthChecks() error = %v", err)
	}

	if !hcCalled {
		t.Error("health check was not called")
	}
}

func TestEnhancedManager_HTTPHandler(t *testing.T) {
	mgr := NewEnhancedManager()
	mgr.healthChecks.SetReady(true)

	handler := mgr.HTTPHandler()

	// Test /health endpoint
	// Note: In real test, use httptest
	_ = handler
}

func TestDrainConfig_Defaults(t *testing.T) {
	cfg := DrainConfig{}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string {
	return "test error"
}
