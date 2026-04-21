// Package openapi provides OpenAPI 3.0 specification generation.
//
// This package helps generate OpenAPI 3.0 specifications for HTTP APIs.
// It provides a builder for defining routes, parameters, and schemas.
//
// # Features
//
//   - OpenAPI 3.0 spec generation
//   - Route and endpoint definitions
//   - Schema definitions
//   - JSON and YAML output
//
// # Usage
//
//	spec := openapi.New("My API", "1.0.0")
//	spec.AddRoute(http.MethodGet, "/users", "List users")
//	spec.AddRoute(http.MethodPost, "/users", "Create user")
//
//	json, _ := spec.ToJSON()
package openapi

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Spec represents an OpenAPI specification
type Spec struct {
	OpenAPI    string              `json:"openapi" yaml:"openapi"`
	Info       Info                `json:"info" yaml:"info"`
	Paths      map[string]PathItem `json:"paths" yaml:"paths"`
	Components Components          `json:"components" yaml:"components"`
	Servers    []Server            `json:"servers,omitempty" yaml:"servers,omitempty"`
}

// Info provides API metadata
type Info struct {
	Title       string `json:"title" yaml:"title"`
	Version     string `json:"version" yaml:"version"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// PathItem represents a path with operations
type PathItem struct {
	GET    *Operation `json:"get,omitempty" yaml:"get,omitempty"`
	POST   *Operation `json:"post,omitempty" yaml:"post,omitempty"`
	PUT    *Operation `json:"put,omitempty" yaml:"put,omitempty"`
	DELETE *Operation `json:"delete,omitempty" yaml:"delete,omitempty"`
	PATCH  *Operation `json:"patch,omitempty" yaml:"patch,omitempty"`
}

// Operation represents an HTTP operation
type Operation struct {
	Summary     string              `json:"summary,omitempty" yaml:"summary"`
	Description string              `json:"description,omitempty" yaml:"description"`
	OperationID string              `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses" yaml:"responses"`
	Tags        []string            `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// Parameter represents an API parameter
type Parameter struct {
	Name        string `json:"name" yaml:"name"`
	In          string `json:"in" yaml:"in"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool   `json:"required,omitempty" yaml:"required,omitempty"`
	Schema      Schema `json:"schema" yaml:"schema"`
}

// RequestBody represents a request body
type RequestBody struct {
	Description string               `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool                 `json:"required,omitempty" yaml:"required,omitempty"`
	Content     map[string]MediaType `json:"content" yaml:"content"`
}

// Response represents an API response
type Response struct {
	Description string               `json:"description" yaml:"description"`
	Content     map[string]MediaType `json:"content,omitempty" yaml:"content,omitempty"`
}

// MediaType represents a media type
type MediaType struct {
	Schema *Schema `json:"schema" yaml:"schema"`
}

// Schema represents a JSON schema
type Schema struct {
	Type       string            `json:"type,omitempty" yaml:"type,omitempty"`
	Properties map[string]Schema `json:"properties,omitempty" yaml:"properties,omitempty"`
	Items      *Schema           `json:"items,omitempty" yaml:"items,omitempty"`
	Required   []string          `json:"required,omitempty" yaml:"required,omitempty"`
	Ref        string            `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Format     string            `json:"format,omitempty" yaml:"format,omitempty"`
	Example    interface{}       `json:"example,omitempty" yaml:"example,omitempty"`
}

// Components holds reusable components
type Components struct {
	Schemas         map[string]Schema         `json:"schemas,omitempty" yaml:"schemas,omitempty"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty" yaml:"securitySchemes,omitempty"`
}

// SecurityScheme represents a security scheme
type SecurityScheme struct {
	Type         string `json:"type" yaml:"type"`
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	Name         string `json:"name,omitempty" yaml:"name,omitempty"`
	In           string `json:"in,omitempty" yaml:"in,omitempty"`
	Scheme       string `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty" yaml:"bearerFormat,omitempty"`
}

// Server represents a server
type Server struct {
	URL         string `json:"url" yaml:"url"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// New creates a new OpenAPI specification
//
//	spec := openapi.New("My API", "1.0.0")
func New(title, version string) *Spec {
	return &Spec{
		OpenAPI: "3.0.3",
		Info: Info{
			Title:   title,
			Version: version,
		},
		Paths: make(map[string]PathItem),
		Components: Components{
			Schemas: make(map[string]Schema),
		},
	}
}

// SetDescription sets API description
func (s *Spec) SetDescription(desc string) {
	s.Info.Description = desc
}

// AddServer adds a server
func (s *Spec) AddServer(url, description string) {
	s.Servers = append(s.Servers, Server{
		URL:         url,
		Description: description,
	})
}

// AddRoute adds a route
//
//	spec.AddRoute(http.MethodGet, "/users", "List users")
//	spec.AddRoute(http.MethodPost, "/users", "Create user")
func (s *Spec) AddRoute(method, path, summary string) *Operation {
	method = strings.ToUpper(method)

	pathItem := s.Paths[path]
	operation := &Operation{
		Summary:   summary,
		Responses: make(map[string]Response),
	}

	switch method {
	case "GET":
		pathItem.GET = operation
	case "POST":
		pathItem.POST = operation
	case "PUT":
		pathItem.PUT = operation
	case "DELETE":
		pathItem.DELETE = operation
	case "PATCH":
		pathItem.PATCH = operation
	}

	s.Paths[path] = pathItem
	return operation
}

// AddSchema adds a reusable schema
func (s *Spec) AddSchema(name string, schema Schema) {
	s.Components.Schemas[name] = schema
}

// AddSecurityScheme adds a security scheme
func (s *Spec) AddSecurityScheme(name string, scheme SecurityScheme) {
	if s.Components.SecuritySchemes == nil {
		s.Components.SecuritySchemes = make(map[string]SecurityScheme)
	}
	s.Components.SecuritySchemes[name] = scheme
}

// AddParam adds a parameter to an operation
func (o *Operation) AddParam(name, in, description string, required bool, schema Schema) *Parameter {
	param := Parameter{
		Name:        name,
		In:          in,
		Description: description,
		Required:    required,
		Schema:      schema,
	}
	o.Parameters = append(o.Parameters, param)
	return &o.Parameters[len(o.Parameters)-1]
}

// SetRequestBody sets the request body
func (o *Operation) SetRequestBody(description string, required bool, schema Schema) *Operation {
	o.RequestBody = &RequestBody{
		Description: description,
		Required:    required,
		Content: map[string]MediaType{
			"application/json": {
				Schema: &schema,
			},
		},
	}
	return o
}

// AddResponse adds a response
func (o *Operation) AddResponse(code int, description string) {
	key := fmt.Sprintf("%d", code)
	o.Responses[key] = Response{
		Description: description,
	}
}

// AddResponseWithSchema adds a response with schema
func (o *Operation) AddResponseWithSchema(code int, description string, schema Schema) {
	key := fmt.Sprintf("%d", code)
	o.Responses[key] = Response{
		Description: description,
		Content: map[string]MediaType{
			"application/json": {
				Schema: &schema,
			},
		},
	}
}

// AddTag adds a tag to operation
func (o *Operation) AddTag(tag string) *Operation {
	o.Tags = append(o.Tags, tag)
	return o
}

// SetOperationID sets the operation ID
func (o *Operation) SetOperationID(id string) *Operation {
	o.OperationID = id
	return o
}

// ToJSON converts spec to JSON
func (s *Spec) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// ToYAML converts spec to YAML
func (s *Spec) ToYAML() ([]byte, error) {
	return yaml.Marshal(s)
}

// String returns JSON string
func (s *Spec) String() string {
	data, _ := s.ToJSON()
	return string(data)
}

// Helper functions for common schemas

// StringSchema returns a string schema
func StringSchema(format string) Schema {
	return Schema{
		Type:   "string",
		Format: format,
	}
}

// IntSchema returns an integer schema
func IntSchema() Schema {
	return Schema{
		Type: "integer",
	}
}

// BoolSchema returns a boolean schema
func BoolSchema() Schema {
	return Schema{
		Type: "boolean",
	}
}

// ArraySchema returns an array schema
func ArraySchema(items Schema) Schema {
	return Schema{
		Type:  "array",
		Items: &items,
	}
}

// ObjectSchema returns an object schema
func ObjectSchema(properties map[string]Schema, required []string) Schema {
	return Schema{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}
}
