package grpcgateway

import (
	"testing"
)

func TestNew(t *testing.T) {
	gw := New()
	if gw == nil {
		t.Fatal("New returned nil")
	}
}

func TestWithEndpoint(t *testing.T) {
	gw := New(WithEndpoint("localhost:9090"))
	if gw.endpoint != "localhost:9090" {
		t.Errorf("endpoint = %s, want localhost:9090", gw.endpoint)
	}
}

func TestRegisterMethod(t *testing.T) {
	gw := New()
	mapping := gw.RegisterMethod("/YourService/GetUser", "GET", "Get user")

	if mapping == nil {
		t.Fatal("RegisterMethod returned nil")
	}
	if mapping.GRPCService != "YourService" {
		t.Errorf("service = %s, want YourService", mapping.GRPCService)
	}
	if mapping.GRPCMethod != "GetUser" {
		t.Errorf("method = %s, want GetUser", mapping.GRPCMethod)
	}
	if mapping.HTTPMethod != "GET" {
		t.Errorf("httpMethod = %s, want GET", mapping.HTTPMethod)
	}
}

func TestListMethods(t *testing.T) {
	gw := New()
	gw.RegisterMethod("/Service1/Method1", "GET", "Method 1")
	gw.RegisterMethod("/Service2/Method2", "POST", "Method 2")

	methods := gw.ListMethods()
	if len(methods) != 2 {
		t.Errorf("expected 2 methods, got %d", len(methods))
	}
}

func TestHealthCheck_NoEndpoint(t *testing.T) {
	gw := New()
	err := gw.HealthCheck(nil)
	if err != nil {
		t.Errorf("HealthCheck error = %v", err)
	}
}

func TestClose(t *testing.T) {
	gw := New(WithEndpoint("localhost:9090"))
	err := gw.Close()
	if err != nil {
		t.Errorf("Close error = %v", err)
	}
}

func TestMapping(t *testing.T) {
	gw := New()

	// Test GET
	m1 := gw.RegisterMethod("/users", "GET", "List users")
	if m1.HTTPPath != "/users" {
		t.Errorf("path = %s, want /users", m1.HTTPPath)
	}

	// Test POST
	m2 := gw.RegisterMethod("/users", "POST", "Create user")
	if m2.HTTPPath != "/users" {
		t.Errorf("path = %s, want /users", m2.HTTPPath)
	}
}
