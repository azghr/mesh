package graphql

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEventStream(t *testing.T) {
	stream := NewEventStream("test-channel")

	assert.Equal(t, "test-channel", stream.Channel)

	stream.Send("test event")

	select {
	case event := <-stream.Events:
		assert.Equal(t, "test event", event)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	stream.Close()
}

func TestEventStream_Close(t *testing.T) {
	stream := NewEventStream("test")
	stream.Close()

	select {
	case _, ok := <-stream.Events:
		assert.False(t, ok, "channel should be closed")
	case <-time.After(time.Millisecond * 100):
	}
}

func TestSubscriptionManager(t *testing.T) {
	sm := NewSubscriptionManager()

	stream := NewEventStream("channel1")
	sm.Subscribe(context.Background(), "channel1", stream)

	sm.Publish("channel1", "event1")

	select {
	case event := <-stream.Events:
		assert.Equal(t, "event1", event)
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	sm.Unsubscribe("channel1", stream)
}

func TestSubscriptionManager_RegisterResolver(t *testing.T) {
	sm := NewSubscriptionManager()

	sm.RegisterResolver("users", func(ctx context.Context, channel string) (*EventStream, error) {
		return NewEventStream(channel), nil
	})

	stream, err := sm.resolvers["users"](context.Background(), "users")
	assert.NoError(t, err)
	assert.NotNil(t, stream)
}

func TestSubscriptionManager_PublishMultiple(t *testing.T) {
	sm := NewSubscriptionManager()

	stream1 := NewEventStream("channel")
	stream2 := NewEventStream("channel")

	sm.Subscribe(context.Background(), "channel", stream1)
	sm.Subscribe(context.Background(), "channel", stream2)

	sm.Publish("channel", "event")

	select {
	case e := <-stream1.Events:
		assert.Equal(t, "event", e)
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	select {
	case e := <-stream2.Events:
		assert.Equal(t, "event", e)
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestResolverBuilder(t *testing.T) {
	rb := NewResolverBuilder()

	rb.Add("user", func(ctx context.Context, source any, args map[string]any) (any, error) {
		return "john", nil
	})

	resolvers := rb.Build()
	assert.Len(t, resolvers, 1)

	val, err := resolvers["user"](context.Background(), nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "john", val)
}

func TestResolverBuilder_Chained(t *testing.T) {
	rb := NewResolverBuilder().
		Add("user", func(ctx context.Context, source any, args map[string]any) (any, error) {
			return "john", nil
		}).
		Add("admin", func(ctx context.Context, source any, args map[string]any) (any, error) {
			return "admin", nil
		})

	resolvers := rb.Build()
	assert.Len(t, resolvers, 2)
}

func TestAsString(t *testing.T) {
	str, ok := AsString("hello")
	assert.True(t, ok)
	assert.Equal(t, "hello", str)

	_, ok = AsString(123)
	assert.False(t, ok)
}

func TestAsInt(t *testing.T) {
	n, ok := AsInt(42)
	assert.True(t, ok)
	assert.Equal(t, 42, n)

	n, ok = AsInt(int64(42))
	assert.True(t, ok)
	assert.Equal(t, 42, n)

	n, ok = AsInt(3.14)
	assert.True(t, ok)
	assert.Equal(t, 3, n)

	_, ok = AsInt("invalid")
	assert.False(t, ok)
}

func TestAsFloat(t *testing.T) {
	n, ok := AsFloat(3.14)
	assert.True(t, ok)
	assert.Equal(t, 3.14, n)

	n, ok = AsFloat(42)
	assert.True(t, ok)
	assert.Equal(t, 42.0, n)
}

func TestAsBool(t *testing.T) {
	b, ok := AsBool(true)
	assert.True(t, ok)
	assert.Equal(t, true, b)

	_, ok = AsBool("yes")
	assert.False(t, ok)
}

func TestAsSlice(t *testing.T) {
	slice, ok := AsSlice([]any{1, 2, 3})
	assert.True(t, ok)
	assert.Len(t, slice, 3)

	_, ok = AsSlice("not a slice")
	assert.False(t, ok)
}

func TestAsMap(t *testing.T) {
	m, ok := AsMap(map[string]any{"key": "value"})
	assert.True(t, ok)
	assert.Equal(t, "value", m["key"])

	_, ok = AsMap("not a map")
	assert.False(t, ok)
}

func TestDirectiveInclude(t *testing.T) {
	result := DirectiveInclude.ResolveDirective(context.Background(), map[string]any{"if": true})
	assert.True(t, result)

	result = DirectiveInclude.ResolveDirective(context.Background(), map[string]any{"if": false})
	assert.False(t, result)
}

func TestDirectiveSkip(t *testing.T) {
	result := DirectiveSkip.ResolveDirective(context.Background(), map[string]any{"if": true})
	assert.False(t, result, "should skip when if=true")

	result = DirectiveSkip.ResolveDirective(context.Background(), map[string]any{"if": false})
	assert.True(t, result, "should not skip when if=false")
}

func TestDirectiveDeprecated(t *testing.T) {
	result := DirectiveDeprecated.ResolveDirective(context.Background(), map[string]any{"reason": "use new field"})
	assert.True(t, result)
}
