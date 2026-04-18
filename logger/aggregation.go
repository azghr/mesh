// Package logger provides structured logging for services.
//
// This package provides log aggregation for shipping logs to central services.
//
// Example:
//
//	logger := logger.New("service", "info", false)
//	logger.SetAggregator(logger.AggregatorConfig{
//	    Endpoint:     "https://logs.example.com/api/v1/logs",
//	    BatchSize:  100,
//	    FlushInterval: 5*time.Second,
//	})
package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// AggregatorConfig holds configuration for log aggregation.
type AggregatorConfig struct {
	Endpoint      string        // URL to send logs
	BatchSize     int           // Max logs per batch
	FlushInterval time.Duration // Max time between flushes
	RetryConfig                 // Retry settings
}

// RetryConfig holds retry configuration.
type RetryConfig struct {
	MaxRetries int           // Max retry attempts, default 3
	Backoff    time.Duration // Backoff between retries, default 1s
}

// AggregationClient handles batching and shipping logs.
type AggregationClient struct {
	client *http.Client
	config AggregatorConfig
	buffer []LogEntry
	mu     sync.Mutex
	ticker *time.Ticker
	stop   chan struct{}
}

// LogEntry represents a single log entry.
type LogEntry struct {
	Time    time.Time      `json:"time"`
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Attrs   map[string]any `json:"attrs,omitempty"`
}

var (
	globalAgg *AggregationClient
	globalMu  sync.Mutex
)

// SetAggregator enables log aggregation.
// Ships logs in batches to reduce network overhead.
func SetAggregator(cfg AggregatorConfig) {
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.Backoff == 0 {
		cfg.Backoff = time.Second
	}

	a := &AggregationClient{
		client: &http.Client{Timeout: 10 * time.Second},
		config: cfg,
		buffer: make([]LogEntry, 0, cfg.BatchSize),
		stop:   make(chan struct{}),
	}

	globalMu.Lock()
	globalAgg = a
	globalMu.Unlock()

	a.ticker = time.NewTicker(cfg.FlushInterval)
	go a.run()
}

// run handles periodic flushes.
func (a *AggregationClient) run() {
	for {
		select {
		case <-a.ticker.C:
			a.flush()
		case <-a.stop:
			a.flush()
			return
		}
	}
}

// flush sends buffered logs to the aggregation service.
func (a *AggregationClient) flush() {
	a.mu.Lock()
	if len(a.buffer) == 0 {
		a.mu.Unlock()
		return
	}

	data := make([]LogEntry, len(a.buffer))
	copy(data, a.buffer)
	a.buffer = a.buffer[:0]
	a.mu.Unlock()

	a.sendWithRetry(data)
}

func (a *AggregationClient) sendWithRetry(data []LogEntry) {
	var err error
	for i := 0; i < a.config.MaxRetries; i++ {
		if err = a.send(data); err == nil {
			return
		}
		time.Sleep(a.config.Backoff * time.Duration(i+1))
	}
	if err != nil {
		fmt.Printf("logger: aggregation error: %v\n", err)
	}
}

func (a *AggregationClient) send(data []LogEntry) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, a.config.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// StopAggregation stops the aggregation client and flushes remaining logs.
func StopAggregation() {
	globalMu.Lock()
	if globalAgg != nil {
		globalAgg.stop <- struct{}{}
		globalAgg.ticker.Stop()
		globalAgg = nil
	}
	globalMu.Unlock()
}
