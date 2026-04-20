# ID Generator

Distributed ID generation using the Snowflake algorithm.

## Overview

This package generates unique 64-bit IDs across distributed systems. Each ID contains:
- Timestamp (41 bits) - milliseconds since custom epoch
- Machine ID (10 bits) - unique machine/node identifier
- Sequence (12 bits) - per-machine counter

## Usage

### Basic Usage

```go
import "github.com/azghr/mesh/idgen"

// Create generator with machine ID (0-1023)
gen, err := idgen.New(1)
if err != nil {
    log.Fatal(err)
}

// Generate unique ID
id := gen.Next()
```

### Global Generator

For simple applications, use the global generator:

```go
// Initialize at startup
err := idgen.InitializeGlobal(1)
if err != nil {
    log.Fatal(err)
}

// Use anywhere
id := idgen.NextID()
```

## API

### `New(machineID int64) (*Generator, error)`

Creates a new generator. Machine ID must be between 0 and 1023.

### `gen.Next() int64`

Returns a unique 64-bit ID.

### `gen.NextUint64() uint64`

Returns the ID as unsigned integer.

### `gen.Info(id) Info`

Breaks down an ID into its components:

```go
info := gen.Info(id)
fmt.Printf("Timestamp: %d, Machine: %d, Sequence: %d\n", 
    info.Timestamp, info.MachineID, info.Sequence)
```

## Configuration

```go
// Custom epoch (default: 2024-01-01)
gen, _ := idgen.New(1, idgen.WithEpoch(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)))
```

## Performance

- ~45ns per ID generation
- Thread-safe for concurrent use
- No allocations after initialization

## Example

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/azghr/mesh/idgen"
)

func main() {
    gen, err := idgen.New(1)
    if err != nil {
        log.Fatal(err)
    }
    
    for i := 0; i < 5; i++ {
        fmt.Printf("ID: %d\n", gen.Next())
    }
    
    // With string output
    fmt.Printf("String: %s\n", gen.NextString())
}
```