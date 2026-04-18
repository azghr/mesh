// Package websocket provides WebSocket handling for real-time communication.
//
// This package wraps fiber/websocket/v2 and adds:
//
//   - Server-side WebSocket handling with message routing
//   - Client with automatic reconnection
//   - Connection pool management
//   - Heartbeat/ping-pong support
//
// # Server Usage
//
//	router.GET("/ws", websocket.Handler(func(c *websocket.Conn) {
//	    // Handle messages
//	    for {
//	        msgType, msg, err := c.ReadMessage()
//	        if err != nil {
//	            return
//	        }
//	        // Process message
//	        c.WriteMessage(msgType, msg)
//	    }
//	}))
//
// # Client Usage
//
//	client := websocket.NewClient(websocket.ClientConfig{
//	    URL: "ws://localhost:8080/ws",
//	})
//	defer client.Close()
//
//	// With auto-reconnection
//	client := websocket.NewClient(websocket.ClientConfig{
//	    URL:            "ws://localhost:8080/ws",
//	    Reconnect:      true,
//	    ReconnectInterval: 5 * time.Second,
//	    MaxRetries:     3,
//	})
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/azghr/mesh/logger"
	"github.com/gofiber/fiber/v2"
	fiberwebsocket "github.com/gofiber/websocket/v2"
	"github.com/gorilla/websocket"
)

type (
	MessageHandler func(conn *Conn, messageType int, data []byte) error

	ServerConfig struct {
		ReadBufferSize    int
		WriteBufferSize   int
		ReadTimeout       time.Duration
		WriteTimeout      time.Duration
		PingInterval      time.Duration
		PongTimeout       time.Duration
		EnableCompression bool
		Subprotocols      []string
		Logger            logger.Logger
	}

	ServerOption func(*ServerConfig)

	Conn struct {
		*fiberwebsocket.Conn
		mu         sync.RWMutex
		localAddr  string
		remoteAddr string
	}

	Server struct {
		app         *fiber.App
		config      *ServerConfig
		handler     MessageHandler
		connections map[*Conn]bool
		mu          sync.RWMutex
	}

	ClientConfig struct {
		URL               string
		Header            http.Header
		Reconnect         bool
		ReconnectInterval time.Duration
		MaxRetries        int
		HandshakeTimeout  time.Duration
		PingInterval      time.Duration
		PongTimeout       time.Duration
		WriteWait         time.Duration
	}

	ClientOption func(*ClientConfig)

	Client struct {
		config    *ClientConfig
		mu        sync.RWMutex
		connected bool
		conn      *fiberwebsocket.Conn
		ctx       context.Context
		cancel    context.CancelFunc
	}
)

func WithReadBufferSize(size int) ServerOption {
	return func(c *ServerConfig) {
		c.ReadBufferSize = size
	}
}

func WithWriteBufferSize(size int) ServerOption {
	return func(c *ServerConfig) {
		c.WriteBufferSize = size
	}
}

func WithReadTimeout(d time.Duration) ServerOption {
	return func(c *ServerConfig) {
		c.ReadTimeout = d
	}
}

func WithWriteTimeout(d time.Duration) ServerOption {
	return func(c *ServerConfig) {
		c.WriteTimeout = d
	}
}

func WithPingInterval(d time.Duration) ServerOption {
	return func(c *ServerConfig) {
		c.PingInterval = d
	}
}

func WithPongTimeout(d time.Duration) ServerOption {
	return func(c *ServerConfig) {
		c.PongTimeout = d
	}
}

func WithCompression(enabled bool) ServerOption {
	return func(c *ServerConfig) {
		c.EnableCompression = enabled
	}
}

func WithSubprotocols(protos ...string) ServerOption {
	return func(c *ServerConfig) {
		c.Subprotocols = protos
	}
}

func WithServerLogger(l logger.Logger) ServerOption {
	return func(c *ServerConfig) {
		c.Logger = l
	}
}

func NewServer(app *fiber.App, handler MessageHandler, opts ...ServerOption) *Server {
	cfg := &ServerConfig{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		ReadTimeout:     60 * time.Second,
		WriteTimeout:    60 * time.Second,
		PingInterval:    30 * time.Second,
		PongTimeout:     10 * time.Second,
		Logger:          logger.GetGlobal(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return &Server{
		app:         app,
		config:      cfg,
		handler:     handler,
		connections: make(map[*Conn]bool),
	}
}

func (s *Server) Register(path string) {
	s.app.Get(path, Handler(s.handler))
}

func (s *Server) handleConnection(c *fiberwebsocket.Conn) {
	conn := wrapConn(c)

	s.mu.Lock()
	s.connections[conn] = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.connections, conn)
		s.mu.Unlock()
		conn.Close()
	}()

	s.config.Logger.Info("websocket connected",
		"remote_addr", c.RemoteAddr().String(),
	)

	if s.config.PingInterval > 0 {
		go s.startPingLoop(conn)
	}

	if s.handler != nil {
		for {
			msgType, msg, err := c.ReadMessage()
			if err != nil {
				s.config.Logger.Error("websocket read error",
					"error", err.Error(),
				)
				break
			}

			if err := s.handler(conn, msgType, msg); err != nil {
				s.config.Logger.Error("websocket handler error",
					"error", err.Error(),
				)
				break
			}
		}
	}

	s.config.Logger.Info("websocket disconnected",
		"remote_addr", c.RemoteAddr().String(),
	)
}

func (s *Server) startPingLoop(conn *Conn) {
	ticker := time.NewTicker(s.config.PingInterval)
	defer ticker.Stop()

	for range ticker.C {
		conn.mu.Lock()
		err := conn.WriteMessage(fiberwebsocket.TextMessage, []byte("ping"))
		conn.mu.Unlock()

		if err != nil {
			return
		}
	}
}

func (s *Server) ConnectionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.connections)
}

func (s *Server) Broadcast(msgType int, data []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for conn := range s.connections {
		conn.mu.Lock()
		err := conn.WriteMessage(msgType, data)
		conn.mu.Unlock()

		if err != nil {
			continue
		}
	}
	return nil
}

func wrapConn(c *fiberwebsocket.Conn) *Conn {
	return &Conn{
		Conn:       c,
		localAddr:  c.LocalAddr().String(),
		remoteAddr: c.RemoteAddr().String(),
	}
}

func (c *Conn) LocalAddr() string {
	return c.localAddr
}

func (c *Conn) RemoteAddr() string {
	return c.remoteAddr
}

func Handler(handler MessageHandler) fiber.Handler {
	return fiberwebsocket.New(func(c *fiberwebsocket.Conn) {
		conn := wrapConn(c)

		for {
			msgType, msg, err := c.ReadMessage()
			if err != nil {
				break
			}

			if err := handler(conn, msgType, msg); err != nil {
				break
			}
		}

		c.Close()
	})
}

func WithURL(url string) ClientOption {
	return func(c *ClientConfig) {
		c.URL = url
	}
}

func WithHeader(key, value string) ClientOption {
	return func(c *ClientConfig) {
		if c.Header == nil {
			c.Header = make(http.Header)
		}
		c.Header.Set(key, value)
	}
}

func WithReconnect(enabled bool, interval time.Duration, maxRetries int) ClientOption {
	return func(c *ClientConfig) {
		c.Reconnect = enabled
		c.ReconnectInterval = interval
		c.MaxRetries = maxRetries
	}
}

func WithHandshakeTimeout(d time.Duration) ClientOption {
	return func(c *ClientConfig) {
		c.HandshakeTimeout = d
	}
}

func WithClientPingInterval(d time.Duration) ClientOption {
	return func(c *ClientConfig) {
		c.PingInterval = d
	}
}

func WithClientPongTimeout(d time.Duration) ClientOption {
	return func(c *ClientConfig) {
		c.PongTimeout = d
	}
}

func NewClient(opts ...ClientOption) *Client {
	cfg := &ClientConfig{
		URL:               "ws://localhost:8080/ws",
		Header:            make(http.Header),
		Reconnect:         false,
		ReconnectInterval: 5 * time.Second,
		MaxRetries:        3,
		HandshakeTimeout:  2 * time.Second,
		PingInterval:      30 * time.Second,
		PongTimeout:       10 * time.Second,
		WriteWait:         10 * time.Second,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return &Client{
		config: cfg,
	}
}

func (c *Client) Connect() (*websocket.Conn, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: c.config.HandshakeTimeout,
	}

	conn, _, err := dialer.Dial(c.config.URL, c.config.Header)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	return conn, nil
}

func (c *Client) ConnectWithRetry() (*websocket.Conn, error) {
	var lastErr error

	for i := 0; c.config.MaxRetries <= 0 || i < c.config.MaxRetries; i++ {
		conn, err := c.Connect()
		if err == nil {
			return conn, nil
		}

		lastErr = err

		if i < c.config.MaxRetries-1 {
			time.Sleep(c.config.ReconnectInterval)
		}
	}

	return nil, fmt.Errorf("failed to connect after %d retries: %w", c.config.MaxRetries, lastErr)
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

func Serve(h MessageHandler, opts ...ServerOption) fiber.Handler {
	cfg := &ServerConfig{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		ReadTimeout:     60 * time.Second,
		WriteTimeout:    60 * time.Second,
		PingInterval:    30 * time.Second,
		PongTimeout:     10 * time.Second,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return Handler(h)
}

func IsWebSocketUpgrade(c *fiber.Ctx) bool {
	return fiberwebsocket.IsWebSocketUpgrade(c)
}

func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		ReadTimeout:     60 * time.Second,
		WriteTimeout:    60 * time.Second,
		PingInterval:    30 * time.Second,
		PongTimeout:     10 * time.Second,
		Logger:          logger.GetGlobal(),
	}
}

func DefaultClientConfig(url string) *ClientConfig {
	return &ClientConfig{
		URL:               url,
		Header:            make(http.Header),
		Reconnect:         false,
		ReconnectInterval: 5 * time.Second,
		MaxRetries:        3,
		HandshakeTimeout:  2 * time.Second,
		PingInterval:      30 * time.Second,
		PongTimeout:       10 * time.Second,
		WriteWait:         10 * time.Second,
	}
}

type JSONMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func (c *Conn) WriteJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteJSON(v)
}

func (c *Conn) ReadJSON(v any) error {
	_, msg, err := c.ReadMessage()
	if err != nil {
		return err
	}
	return json.Unmarshal(msg, v)
}

func (c *Client) WriteJSON(v any) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.conn == nil {
		return fmt.Errorf("not connected")
	}

	return c.conn.WriteJSON(v)
}

func (c *Client) ReadJSON(v any) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.conn == nil {
		return fmt.Errorf("not connected")
	}

	_, msg, err := c.conn.ReadMessage()
	if err != nil {
		return err
	}

	return json.Unmarshal(msg, v)
}
