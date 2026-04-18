# WebSocket

WebSocket handling for real-time communication with Fiber/gorilla websocket.

## What It Does

Provides WebSocket server and client with:

- **Server**: Message handling with connection pool
- **Client**: Auto-reconnection support
- **Ping/Pong**: Heartbeat support
- **Broadcasting**: Send to all connected clients

## Quick Start

### Server

```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/azghr/mesh/websocket"
)

app := fiber.New()

app.Get("/ws", websocket.Handler(func(conn *websocket.Conn, msgType int, data []byte) error {
    // Echo back
    return conn.WriteMessage(msgType, data)
}))

app.Listen(":8080")
```

### Client

```go
client := websocket.NewClient(
    websocket.WithURL("ws://localhost:8080/ws"),
)
defer client.Close()

conn, err := client.Connect()
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

// Send message
conn.WriteMessage(websocket.TextMessage, []byte("hello"))

// Read message
_, msg, err := conn.ReadMessage()
```

## Server

### Basic Usage

```go
app.Get("/ws", websocket.Handler(func(c *websocket.Conn, msgType int, data []byte) error {
    switch msgType {
    case websocket.TextMessage:
        // Handle text
    case websocket.BinaryMessage:
        // Handle binary
    case websocket.CloseMessage:
        // Handle close
    }
    return nil
}))
```

### With Options

```go
handler := websocket.Handler(func(c *websocket.Conn, msgType int, data []byte) error {
    // Handle message
    return c.WriteMessage(msgType, data)
},
    websocket.WithReadBufferSize(2048),
    websocket.WithWriteBufferSize(2048),
    websocket.WithPingInterval(30 * time.Second),
)
```

### Server Type

```go
server := websocket.NewServer(app, handleMessage,
    websocket.WithPingInterval(30 * time.Second),
)

server.Register("/ws")

// Broadcast to all clients
server.Broadcast(websocket.TextMessage, []byte("Hello all!"))

// Get connection count
count := server.ConnectionCount()
```

## Client

### Basic Connection

```go
client := websocket.NewClient(
    websocket.WithURL("ws://localhost:8080/ws"),
)

conn, err := client.Connect()
if err != nil {
    log.Fatal(err)
}
defer conn.Close()
```

### With Reconnection

```go
client := websocket.NewClient(
    websocket.WithURL("ws://localhost:8080/ws"),
    websocket.WithReconnect(true, 5 * time.Second, 3),
)

conn, err := client.ConnectWithRetry()
if err != nil {
    log.Fatal(err)
}
```

### With Headers

```go
client := websocket.NewClient(
    websocket.WithURL("ws://localhost:8080/ws"),
    websocket.WithHeader("Authorization", "Bearer token"),
)
```

## Configuration

### Server Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithReadBufferSize` | Read buffer size | 1024 |
| `WithWriteBufferSize` | Write buffer size | 1024 |
| `WithReadTimeout` | Read timeout | 60s |
| `WithWriteTimeout` | Write timeout | 60s |
| `WithPingInterval` | Ping interval | 30s |
| `WithPongTimeout` | Pong timeout | 10s |
| `WithCompression` | Enable compression | false |
| `WithSubprotocols` | Subprotocols | [] |
| `WithServerLogger` | Custom logger | global |

### Client Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithURL` | WebSocket URL | required |
| `WithHeader` | HTTP headers | {} |
| `WithReconnect` | Enable reconnection | false |
| `WithReconnectInterval` | Retry interval | 5s |
| `WithMaxRetries` | Max retries | 3 |
| `WithHandshakeTimeout` | Handshake timeout | 2s |

## Message Types

| Constant | Value | Description |
|----------|-------|-------------|
| `TextMessage` | 1 | Text frame |
| `BinaryMessage` | 2 | Binary frame |
| `CloseMessage` | 8 | Close frame |
| `PingMessage` | 9 | Ping frame |
| `PongMessage` | 10 | Pong frame |

## JSON Messages

### Server JSON

```go
type Message struct {
    Type    string `json:"type"`
    Payload any    `json:"payload"`
}

app.Get("/ws", websocket.Handler(func(c *websocket.Conn, msgType int, data []byte) error {
    var msg Message
    if err := json.Unmarshal(data, &msg); err != nil {
        return err
    }
    
    // Process and respond
    resp := Message{Type: "response", Payload: "ok"}
    return c.WriteJSON(resp)
}))
```

### Client JSON

```go
conn, _ := client.Connect()

msg := Message{Type: "ping", Payload: time.Now()}
if err := conn.WriteJSON(msg); err != nil {
    log.Fatal(err)
}

var resp Message
if err := conn.ReadJSON(&resp); err != nil {
    log.Fatal(err)
}
```

## Connection Pool

### Server with Pool

```go
server := websocket.NewServer(app, handleMessage)

// Register handler
server.Register("/ws")

// Broadcast
go func() {
    for {
        time.Sleep(5 * time.Second)
        server.Broadcast(websocket.TextMessage, []byte(time.Now().String()))
    }
}()

// Get stats
log.Info("connections", "count", server.ConnectionCount())
```

## Heartbeat

### Server Ping

Automatic ping is sent if `PingInterval` is set:

```go
server := websocket.NewServer(app, handler,
    websocket.WithPingInterval(30 * time.Second),
)
```

## Error Handling

### Common Errors

| Error | Cause |
|-------|-------|
| `websocket: close 1001` | Client disconnected |
| `websocket: bad handshake` | Invalid upgrade request |
| `use of closed network connection` | Connection closed |

### Close Codes

```go
switch err {
case websocket.IsCloseError(err, websocket.CloseNormalClosure):
    // Normal close
case websocket.IsCloseError(err, websocket.CloseAbnormalClosure):
    // Abnormal close
default:
    // Other error
}
```

## Helper Functions

### Check WebSocket Upgrade

```go
app.Use(func(c *fiber.Ctx) error {
    if !websocket.IsWebSocketUpgrade(c) {
        return c.Status(400).SendString("Not a WebSocket upgrade")
    }
    return c.Next()
})
```

## Full Example

### Server

```go
package main

import (
    "log"
    "time"

    "github.com/azghr/mesh/websocket"
    "github.com/gofiber/fiber/v2"
    "github.com/gorilla/websocket"
)

func main() {
    app := fiber.New()

    // Simple handler
    app.Get("/ws", websocket.Handler(func(c *websocket.Conn, msgType int, data []byte) error {
        log.Printf("received: %s", string(data))
        
        // Echo with timestamp
        resp := map[string]any{
            "echo":   string(data),
            "time":   time.Now().Unix(),
        }
        return c.WriteJSON(resp)
    }))

    // Or use server for broadcasting
    server := websocket.NewServer(app, handleMessage)
    server.Register("/chat")

    app.Listen(":8080")
}

func handleMessage(c *websocket.Conn, msgType int, data []byte) error {
    if msgType == websocket.CloseMessage {
        return nil
    }
    return c.WriteMessage(msgType, data)
}
```

### Client

```go
package main

import (
    "log"

    "github.com/azghr/mesh/websocket"
    "github.com/gorilla/websocket"
)

func main() {
    client := websocket.NewClient(
        websocket.WithURL("ws://localhost:8080/ws"),
        websocket.WithHeader("X-API-Key", "secret"),
        websocket.WithReconnect(true, 5 * time.Second, 3),
    )

    conn, err := client.ConnectWithRetry()
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    // Send
    if err := conn.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
        log.Fatal(err)
    }

    // Receive
    _, msg, err := conn.ReadMessage()
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("received: %s", string(msg))
}