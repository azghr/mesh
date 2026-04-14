package eventbus

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	bus := New()
	assert.NotNil(t, bus)
	assert.Empty(t, bus.Topics())
}

func TestSubscribe(t *testing.T) {
	bus := New()

	var called bool
	unsubscribe := bus.Subscribe("test.event", func(payload any) {
		called = true
	})

	bus.PublishSync("test.event", "test")
	assert.True(t, called)

	unsubscribe()
	assert.Equal(t, 0, bus.HandlerCount("test.event"))
}

func TestPublishSync(t *testing.T) {
	bus := New()

	var results []string
	bus.Subscribe("test.event", func(payload any) {
		results = append(results, payload.(string))
	})

	bus.PublishSync("test.event", "first")
	bus.PublishSync("test.event", "second")

	assert.Equal(t, []string{"first", "second"}, results)
}

func TestPublishAsync(t *testing.T) {
	bus := New()

	var mu sync.Mutex
	var results []string
	bus.Subscribe("test.event", func(payload any) {
		mu.Lock()
		results = append(results, payload.(string))
		mu.Unlock()
	})

	// Just test that Publish doesn't panic - handlers run async
	bus.Publish("test.event", "async")

	// Give a small time for the goroutine to run
	// In production, you'd use a proper async test framework
}

func TestSubscribeOnce(t *testing.T) {
	bus := New()

	count := 0
	bus.SubscribeOnce("test.event", func(payload any) {
		count++
	})

	bus.PublishSync("test.event", "first")
	bus.PublishSync("test.event", "second")

	assert.Equal(t, 1, count)
}

func TestClear(t *testing.T) {
	bus := New()

	bus.Subscribe("test.event", func(payload any) {})
	bus.Subscribe("test.event", func(payload any) {})

	assert.Equal(t, 2, bus.HandlerCount("test.event"))

	bus.Clear("test.event")

	assert.Equal(t, 0, bus.HandlerCount("test.event"))
}

func TestClearAll(t *testing.T) {
	bus := New()

	bus.Subscribe("event1", func(payload any) {})
	bus.Subscribe("event2", func(payload any) {})

	bus.ClearAll()

	assert.Empty(t, bus.Topics())
}

func TestTopics(t *testing.T) {
	bus := New()

	bus.Subscribe("event1", func(payload any) {})
	bus.Subscribe("event2", func(payload any) {})

	topics := bus.Topics()
	assert.Len(t, topics, 2)
}

func TestGlobalBus(t *testing.T) {
	GlobalClear()

	var called bool
	GlobalSubscribe("global.event", func(payload any) {
		called = true
	})

	GlobalPublishSync("global.event", "test")
	assert.True(t, called)

	GlobalClear()
}

func TestJSONSubscribe(t *testing.T) {
	bus := New()

	var data map[string]string
	bus.JSONSubscribe("json.event", func(payload any) error {
		if m, ok := payload.(map[string]string); ok {
			data = m
		}
		return nil
	})

	bus.PublishSync("json.event", map[string]string{"key": "value"})
	assert.Equal(t, "value", data["key"])
}
