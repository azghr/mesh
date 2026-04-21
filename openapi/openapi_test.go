package openapi

import (
	"testing"
)

func TestNew(t *testing.T) {
	spec := New("My API", "1.0.0")
	if spec == nil {
		t.Fatal("New returned nil")
	}
	if spec.Info.Title != "My API" {
		t.Errorf("title = %s, want My API", spec.Info.Title)
	}
	if spec.Info.Version != "1.0.0" {
		t.Errorf("version = %s, want 1.0.0", spec.Info.Version)
	}
}

func TestAddRoute(t *testing.T) {
	spec := New("Test", "1.0.0")
	op := spec.AddRoute("GET", "/users", "List users")

	if op == nil {
		t.Fatal("AddRoute returned nil")
	}
	if len(spec.Paths) != 1 {
		t.Error("path not added")
	}
}

func TestAddRoute_GET(t *testing.T) {
	spec := New("Test", "1.0.0")
	spec.AddRoute("GET", "/users", "List users")

	if spec.Paths["/users"].GET == nil {
		t.Error("GET operation not added")
	}
}

func TestAddRoute_POST(t *testing.T) {
	spec := New("Test", "1.0.0")
	spec.AddRoute("POST", "/users", "Create user")

	if spec.Paths["/users"].POST == nil {
		t.Error("POST operation not added")
	}
}

func TestOperation_AddParam(t *testing.T) {
	spec := New("Test", "1.0.0")
	op := spec.AddRoute("GET", "/users", "List users")
	op.AddParam("limit", "query", "Limit results", false, IntSchema())

	if len(op.Parameters) != 1 {
		t.Errorf("expected 1 param, got %d", len(op.Parameters))
	}
}

func TestOperation_SetRequestBody(t *testing.T) {
	spec := New("Test", "1.0.0")
	op := spec.AddRoute("POST", "/users", "Create user")
	op.SetRequestBody("User data", true, ObjectSchema(nil, nil))

	if op.RequestBody == nil {
		t.Error("request body not set")
	}
	if !op.RequestBody.Required {
		t.Error("request body should be required")
	}
}

func TestOperation_AddResponse(t *testing.T) {
	spec := New("Test", "1.0.0")
	op := spec.AddRoute("GET", "/users", "List users")
	op.AddResponse(200, "Success")
	op.AddResponse(500, "Error")

	if len(op.Responses) != 2 {
		t.Errorf("expected 2 responses, got %d", len(op.Responses))
	}
}

func TestOperation_AddTag(t *testing.T) {
	spec := New("Test", "1.0.0")
	op := spec.AddRoute("GET", "/users", "List users")
	op.AddTag("users")

	if len(op.Tags) != 1 || op.Tags[0] != "users" {
		t.Error("tag not added")
	}
}

func TestOperation_SetOperationID(t *testing.T) {
	spec := New("Test", "1.0.0")
	op := spec.AddRoute("GET", "/users", "List users")
	op.SetOperationID("listUsers")

	if op.OperationID != "listUsers" {
		t.Errorf("operationId = %s, want listUsers", op.OperationID)
	}
}

func TestSpec_AddSchema(t *testing.T) {
	spec := New("Test", "1.0.0")
	spec.AddSchema("User", ObjectSchema(nil, nil))

	if len(spec.Components.Schemas) != 1 {
		t.Error("schema not added")
	}
}

func TestSpec_AddServer(t *testing.T) {
	spec := New("Test", "1.0.0")
	spec.AddServer("http://localhost:8080", "Local server")

	if len(spec.Servers) != 1 {
		t.Error("server not added")
	}
	if spec.Servers[0].URL != "http://localhost:8080" {
		t.Error("server URL incorrect")
	}
}

func TestSpec_ToJSON(t *testing.T) {
	spec := New("Test", "1.0.0")
	spec.AddRoute("GET", "/health", "Health check")

	data, err := spec.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error = %v", err)
	}

	if len(data) == 0 {
		t.Error("empty JSON output")
	}
}

func TestSpec_ToYAML(t *testing.T) {
	spec := New("Test", "1.0.0")
	spec.AddRoute("GET", "/health", "Health check")

	data, err := spec.ToYAML()
	if err != nil {
		t.Fatalf("ToYAML error = %v", err)
	}

	if len(data) == 0 {
		t.Error("empty YAML output")
	}
}

func TestStringSchema(t *testing.T) {
	s := StringSchema("email")
	if s.Type != "string" {
		t.Errorf("type = %s, want string", s.Type)
	}
	if s.Format != "email" {
		t.Errorf("format = %s, want email", s.Format)
	}
}

func TestIntSchema(t *testing.T) {
	s := IntSchema()
	if s.Type != "integer" {
		t.Errorf("type = %s, want integer", s.Type)
	}
}

func TestBoolSchema(t *testing.T) {
	s := BoolSchema()
	if s.Type != "boolean" {
		t.Errorf("type = %s, want boolean", s.Type)
	}
}

func TestArraySchema(t *testing.T) {
	s := ArraySchema(StringSchema(""))
	if s.Type != "array" {
		t.Errorf("type = %s, want array", s.Type)
	}
	if s.Items == nil {
		t.Error("items not set")
	}
}

func TestObjectSchema(t *testing.T) {
	props := map[string]Schema{
		"name": StringSchema(""),
		"age":  IntSchema(),
	}
	s := ObjectSchema(props, []string{"name"})
	if s.Type != "object" {
		t.Errorf("type = %s, want object", s.Type)
	}
	if len(s.Required) != 1 || s.Required[0] != "name" {
		t.Error("required not set correctly")
	}
}
