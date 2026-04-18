package graphql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSchema(t *testing.T) {
	schema := NewSchema(SchemaConfig{
		Query: NewObject("Query", []Field{
			{Name: "hello", Type: "String", Resolve: func(ctx context.Context, src any, args map[string]any) (any, error) {
				return "world", nil
			}},
		}),
	})

	assert.NotNil(t, schema)
}

func TestExecQuery(t *testing.T) {
	schema := NewSchema(SchemaConfig{
		Query: NewObject("Query", []Field{
			{Name: "user", Type: "String", Resolve: func(ctx context.Context, src any, args map[string]any) (any, error) {
				return "john", nil
			}},
		}),
	})

	result := schema.Exec(context.Background(), "{ user }", nil)

	assert.NotNil(t, result)
	assert.Equal(t, "john", result.Data["user"])
}

func TestExecWithVariables(t *testing.T) {
	schema := NewSchema(SchemaConfig{
		Query: NewObject("Query", []Field{
			{Name: "greet", Type: "String", Args: []InputValue{{Name: "name", Type: "String"}}, Resolve: func(ctx context.Context, src any, args map[string]any) (any, error) {
				name, _ := args["name"].(string)
				return "Hello, " + name, nil
			}},
		}),
	})

	result := schema.Exec(context.Background(), "{ greet(name: \"John\") }", nil)

	assert.NotNil(t, result)
}

func TestMutation(t *testing.T) {
	createUser := func(ctx context.Context, src any, args map[string]any) (any, error) {
		return map[string]any{"id": "1", "name": "John"}, nil
	}

	schema := NewSchema(SchemaConfig{
		Query: NewObject("Query", []Field{
			{Name: "user", Type: "String", Resolve: func(ctx context.Context, src any, args map[string]any) (any, error) {
				return "john", nil
			}},
		}),
		Mutation: NewObject("Mutation", []Field{
			{Name: "createUser", Type: "User", Resolve: createUser},
		}),
	})

	result := schema.Exec(context.Background(), "mutation { createUser { id name } }", nil)

	assert.NotNil(t, result)
}

func TestIntrospect(t *testing.T) {
	schema := NewSchema(SchemaConfig{
		Query: NewObject("Query", []Field{
			{Name: "user", Type: "String", Resolve: func(ctx context.Context, src any, args map[string]any) (any, error) {
				return "john", nil
			}},
		}),
	})

	intro := schema.Introspect()

	assert.NotNil(t, intro)
	assert.Contains(t, intro, "query")
}

func TestNonNull(t *testing.T) {
	assert.Equal(t, "String!", NonNull("String"))
}

func TestList(t *testing.T) {
	assert.Equal(t, "[String]", List("String"))
}

func TestInputValue(t *testing.T) {
	arg := InputValueDef("name", "String", "default")
	assert.Equal(t, "name", arg.Name)
	assert.Equal(t, "String", arg.Type)
}

func TestIsNonNullType(t *testing.T) {
	assert.True(t, IsNonNullType("String!"))
	assert.False(t, IsNonNullType("String"))
}

func TestIsListType(t *testing.T) {
	assert.True(t, IsListType("[String]"))
	assert.False(t, IsListType("String"))
}

func TestGetTypeName(t *testing.T) {
	assert.Equal(t, "String", GetTypeName("String!"))
	assert.Equal(t, "String", GetTypeName("String"))
	assert.Equal(t, "String", GetTypeName("[String]"))
}

func TestResolveHelper(t *testing.T) {
	field := Resolve("user", "String", func(ctx context.Context, src any, args map[string]any) (any, error) {
		return "john", nil
	})

	assert.Equal(t, "user", field.Name)
	assert.Equal(t, "String", field.Type)
	assert.NotNil(t, field.Resolve)
}

func TestScalar(t *testing.T) {
	assert.Equal(t, "Int", Scalar("Int"))
}

func TestFieldWithArgs(t *testing.T) {
	field := Resolve("user", "User", func(ctx context.Context, src any, args map[string]any) (any, error) {
		return nil, nil
	}, InputValueDef("id", "ID", ""))

	assert.Equal(t, "id", field.Args[0].Name)
}

func TestExecutionResult(t *testing.T) {
	result := &ExecutionResult{
		Data: map[string]any{"hello": "world"},
	}

	assert.False(t, result.HasErrors())
	assert.Empty(t, result.Error())
}
