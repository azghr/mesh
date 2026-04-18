package websocket

import (
	"testing"
	"time"

	"github.com/azghr/mesh/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestHandler(t *testing.T) {
	app := fiber.New()

	handler := func(conn *Conn, msgType int, data []byte) error {
		return conn.WriteMessage(msgType, data)
	}

	app.Get("/ws", Handler(handler))

	assert.NotNil(t, app)
}

func TestServerConfig(t *testing.T) {
	cfg := DefaultServerConfig()

	assert.Equal(t, 1024, cfg.ReadBufferSize)
	assert.Equal(t, 1024, cfg.WriteBufferSize)
	assert.Equal(t, 60*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.PingInterval)
}

func TestClientConfig(t *testing.T) {
	cfg := DefaultClientConfig("ws://localhost:8080/ws")

	assert.Equal(t, "ws://localhost:8080/ws", cfg.URL)
	assert.Equal(t, 5*time.Second, cfg.ReconnectInterval)
	assert.Equal(t, 3, cfg.MaxRetries)
}

func TestIsWebSocketUpgrade(t *testing.T) {
	app := fiber.New()

	app.Get("/test", func(c *fiber.Ctx) error {
		isWS := IsWebSocketUpgrade(c)
		assert.False(t, isWS)
		return c.SendStatus(200)
	})

	assert.NotNil(t, app)
}

func TestServerOptions(t *testing.T) {
	app := fiber.New()
	l := logger.GetGlobal()

	server := NewServer(app, nil,
		WithReadBufferSize(2048),
		WithWriteBufferSize(2048),
		WithReadTimeout(30*time.Second),
		WithWriteTimeout(30*time.Second),
		WithPingInterval(15*time.Second),
		WithPongTimeout(5*time.Second),
		WithCompression(true),
		WithSubprotocols("json", "protobuf"),
		WithServerLogger(l),
	)

	assert.Equal(t, 2048, server.config.ReadBufferSize)
	assert.Equal(t, 2048, server.config.WriteBufferSize)
	assert.Equal(t, 30*time.Second, server.config.ReadTimeout)
	assert.Equal(t, 30*time.Second, server.config.WriteTimeout)
	assert.Equal(t, 15*time.Second, server.config.PingInterval)
	assert.Equal(t, 5*time.Second, server.config.PongTimeout)
	assert.True(t, server.config.EnableCompression)
	assert.Contains(t, server.config.Subprotocols, "json")
	assert.Contains(t, server.config.Subprotocols, "protobuf")
}

func TestClientOptions(t *testing.T) {
	l := NewClient(
		WithURL("ws://example.com/ws"),
		WithHeader("Authorization", "Bearer test"),
		WithReconnect(true, 10*time.Second, 5),
		WithHandshakeTimeout(5*time.Second),
		WithClientPingInterval(20*time.Second),
		WithClientPongTimeout(8*time.Second),
	)

	assert.Equal(t, "ws://example.com/ws", l.config.URL)
	assert.Equal(t, "Bearer test", l.config.Header.Get("Authorization"))
	assert.True(t, l.config.Reconnect)
	assert.Equal(t, 10*time.Second, l.config.ReconnectInterval)
	assert.Equal(t, 5, l.config.MaxRetries)
	assert.Equal(t, 5*time.Second, l.config.HandshakeTimeout)
	assert.Equal(t, 20*time.Second, l.config.PingInterval)
	assert.Equal(t, 8*time.Second, l.config.PongTimeout)
}

func TestMessageHandlerSignature(t *testing.T) {
	var handler MessageHandler = func(conn *Conn, messageType int, data []byte) error {
		assert.NotNil(t, conn)
		return nil
	}

	assert.NotNil(t, handler)
}

func TestConnHelpers(t *testing.T) {
	app := fiber.New()

	app.Get("/ws", Handler(func(c *Conn, msgType int, data []byte) error {
		assert.Equal(t, "127.0.0.1", c.RemoteAddr())
		assert.Equal(t, "127.0.0.1", c.LocalAddr())
		return nil
	}))

	assert.NotNil(t, app)
}
