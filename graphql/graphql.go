// Package graphql provides GraphQL schema and resolver helpers.
//
// This package provides a simple GraphQL execution layer:
//
//   - Define schema with Query/Mutation types
//   - Resolve fields with functions
//   - Execute queries against the schema
//
// # Quick Start
//
//	schema := graphql.Schema(graphql.SchemaConfig{
//	    Query: graphql.Object("Query", []graphql.Field{
//	        {Name: "user", Type: "User", Resolve: getUser},
//	    }),
//	    Mutation: graphql.Object("Mutation", []graphql.Field{
//	        {Name: "createUser", Type: "User", Resolve: createUser},
//	    }),
//	})
//
//	result, err := schema.Exec(ctx, query, variables)
package graphql

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/azghr/mesh/logger"
)

type (
	Schema struct {
		queryType    *Object
		mutationType *Object
		logger       logger.Logger
		fields       map[string]*Field
		mu           sync.RWMutex
	}

	SchemaConfig struct {
		Query    *Object
		Mutation *Object
		Types    []*Object
	}

	Object struct {
		Name       string
		Fields     []Field
		Interfaces []string
	}

	Field struct {
		Name        string
		Description string
		Type        string
		Args        []InputValue
		Resolve     Resolver
		IsNonNull   bool
		IsList      bool
	}

	InputValue struct {
		Name         string
		Type         string
		DefaultValue any
		Description  string
	}

	Resolver func(ctx context.Context, source any, args map[string]any) (any, error)

	ExecutionResult struct {
		Data   map[string]any
		Errors []GraphQLError
	}

	GraphQLError struct {
		Message   string
		Locations []Location
		Path      []any
	}

	Location struct {
		Line   int
		Column int
	}

	Config struct {
		logger logger.Logger
	}

	Option func(*Config)
)

const (
	TypeString  = "String"
	TypeInt     = "Int"
	TypeFloat   = "Float"
	TypeBoolean = "Boolean"
	TypeID      = "ID"
	TypeList    = "List"
)

func WithLogger(l logger.Logger) Option {
	return func(c *Config) {
		c.logger = l
	}
}

func NewSchema(cfg SchemaConfig, opts ...Option) *Schema {
	s := &Schema{
		queryType:    cfg.Query,
		mutationType: cfg.Mutation,
		logger:       logger.GetGlobal(),
		fields:       make(map[string]*Field),
	}

	if cfg.Query != nil {
		for i := range cfg.Query.Fields {
			field := &cfg.Query.Fields[i]
			s.fields[field.Name] = field
		}
	}
	if cfg.Mutation != nil {
		for i := range cfg.Mutation.Fields {
			field := &cfg.Mutation.Fields[i]
			s.fields[field.Name] = field
		}
	}

	return s
}

func (s *Schema) Exec(ctx context.Context, query string, vars map[string]any) *ExecutionResult {
	if vars == nil {
		vars = make(map[string]any)
	}

	operation, selSet, err := parseQuery(query)
	if err != nil {
		return &ExecutionResult{
			Errors: []GraphQLError{{Message: err.Error()}},
		}
	}

	result := make(map[string]any)

	switch operation {
	case "query":
		result = s.executeQuery(ctx, selSet, vars)
	case "mutation":
		result = s.executeMutation(ctx, selSet, vars)
	default:
		return &ExecutionResult{
			Errors: []GraphQLError{{Message: "unsupported operation type"}},
		}
	}

	return &ExecutionResult{
		Data: result,
	}
}

func (s *Schema) executeQuery(ctx context.Context, selectionSet []string, vars map[string]any) map[string]any {
	result := make(map[string]any)

	for _, fieldName := range selectionSet {
		s.mu.RLock()
		field, ok := s.fields[fieldName]
		s.mu.RUnlock()

		if !ok || field.Resolve == nil {
			result[fieldName] = nil
			continue
		}

		val, err := field.Resolve(ctx, nil, vars)
		if err != nil {
			s.logger.Error("graphql resolve error",
				"field", fieldName,
				"error", err.Error(),
			)
			continue
		}

		result[fieldName] = val
	}

	return result
}

func (s *Schema) executeMutation(ctx context.Context, selectionSet []string, vars map[string]any) map[string]any {
	if s.mutationType == nil {
		return make(map[string]any)
	}
	return s.executeQuery(ctx, selectionSet, vars)
}

func Resolve(name string, fieldType string, resolve Resolver, args ...InputValue) Field {
	return Field{
		Name:    name,
		Type:    fieldType,
		Resolve: resolve,
		Args:    args,
	}
}

func parseQuery(query string) (string, []string, error) {
	query = strings.TrimSpace(query)

	if strings.HasPrefix(query, "query") {
		return "query", parseSelection(strings.TrimPrefix(query, "query")), nil
	}
	if strings.HasPrefix(query, "mutation") {
		return "mutation", parseSelection(strings.TrimPrefix(query, "mutation")), nil
	}

	return "query", parseSelection(query), nil
}

func parseSelection(q string) []string {
	var selections []string

	start := 0
	depth := 0

	for i, ch := range q {
		switch ch {
		case '{':
			if depth == 0 {
				start = i + 1
			}
			depth++
		case '}':
			depth--
			if depth == 0 {
				sel := strings.TrimSpace(q[start:i])
				if sel != "" {
					selections = append(selections, parseFields(sel)...)
				}
			}
		}
	}

	return selections
}

func parseFields(s string) []string {
	var fields []string
	var current strings.Builder

	for _, ch := range s {
		if ch == ' ' || ch == '\n' || ch == '\r' || ch == '\t' {
			if current.Len() > 0 {
				name := strings.TrimSpace(current.String())
				if name != "" && name != "," {
					fields = append(fields, name)
				}
				current.Reset()
			}
			continue
		}
		if ch == '(' || ch == ')' || ch == ',' {
			if current.Len() > 0 {
				name := strings.TrimSpace(current.String())
				if name != "" && name != "," {
					fields = append(fields, name)
				}
				current.Reset()
			}
			continue
		}
		current.WriteRune(ch)
	}

	if current.Len() > 0 {
		name := strings.TrimSpace(current.String())
		if name != "" {
			fields = append(fields, name)
		}
	}

	return fields
}

func NewObject(name string, fields []Field) *Object {
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Name < fields[j].Name
	})
	return &Object{
		Name:   name,
		Fields: fields,
	}
}

func NonNull(t string) string {
	return t + "!"
}

func List(t string) string {
	return "[" + t + "]"
}

func InputValueDef(name, typeName string, defaultVal any) InputValue {
	return InputValue{
		Name:         name,
		Type:         typeName,
		DefaultValue: defaultVal,
	}
}

func (s *Schema) Introspect() map[string]any {
	result := make(map[string]any)

	result["query"] = s.introspectType(s.queryType)
	if s.mutationType != nil {
		result["mutation"] = s.introspectType(s.mutationType)
	}

	return result
}

func (s *Schema) introspectType(obj *Object) map[string]any {
	if obj == nil {
		return nil
	}

	fields := make([]map[string]any, len(obj.Fields))
	for i, f := range obj.Fields {
		fields[i] = map[string]any{
			"name": f.Name,
			"type": f.Type,
			"args": s.introspectArgs(f.Args),
		}
	}

	return map[string]any{
		"name":   obj.Name,
		"fields": fields,
	}
}

func (s *Schema) introspectArgs(args []InputValue) []map[string]any {
	result := make([]map[string]any, len(args))
	for i, a := range args {
		result[i] = map[string]any{
			"name": a.Name,
			"type": a.Type,
		}
	}
	return result
}

func Scalar(t string) string {
	return t
}

func Interface(name string, fields []Field) *Object {
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Name < fields[j].Name
	})
	return &Object{
		Name:   name,
		Fields: fields,
	}
}

func Union(name string, types []string) *Object {
	return &Object{
		Name: name,
	}
}

func IsNonNullType(t string) bool {
	return strings.HasSuffix(t, "!")
}

func IsListType(t string) bool {
	return strings.HasPrefix(t, "[")
}

func GetTypeName(t string) string {
	result := strings.TrimPrefix(t, "[")
	result = strings.TrimSuffix(result, "]")
	result = strings.TrimSuffix(result, "!")
	return result
}

func Arg(name string, t string) InputValue {
	return InputValue{Name: name, Type: t}
}

func (e *ExecutionResult) Error() string {
	if len(e.Errors) == 0 {
		return ""
	}
	return e.Errors[0].Message
}

func (e *ExecutionResult) HasErrors() bool {
	return len(e.Errors) > 0
}
