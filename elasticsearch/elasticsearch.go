// Package elasticsearch provides a simple client for Elasticsearch.
//
// This package provides a lightweight client for Elasticsearch operations
// including indexing, searching, and managing documents.
//
// # Features
//
//   - Simple document indexing
//   - Full-text search
//   - Bulk operations
//   - Document deletion
//
// # Usage
//
//	client, _ := elasticsearch.New(elasticsearch.Config{
//	    Addresses: []string{"http://localhost:9200"},
//	})
//	client.Index(ctx, "users", "1", map[string]string{"name": "John"})
//	client.Search(ctx, "users", "John")
package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Config configures the Elasticsearch client
type Config struct {
	Addresses []string
	Username  string
	Password  string
}

// Client provides Elasticsearch operations
type Client struct {
	config Config
	client *http.Client
}

// New creates a new Elasticsearch client
//
//	client, _ := elasticsearch.New(elasticsearch.Config{
//	    Addresses: []string{"http://localhost:9200"},
//	})
func New(cfg Config) (*Client, error) {
	if len(cfg.Addresses) == 0 {
		return nil, fmt.Errorf("at least one address required")
	}

	return &Client{
		config: cfg,
		client: &http.Client{},
	}, nil
}

// Document represents an Elasticsearch document
type Document struct {
	ID     string          `json:"id,omitempty"`
	Source json.RawMessage `json:"_source,omitempty"`
}

// SearchResult represents search results
type SearchResult struct {
	Total int        `json:"total"`
	Hits  []Document `json:"hits"`
}

// Index indexes a document
//
//	client.Index(ctx, "users", "1", map[string]string{
//	    "name": "John",
//	    "email": "john@example.com",
//	})
func (c *Client) Index(ctx context.Context, index, id string, document interface{}) error {
	url := fmt.Sprintf("%s/%s/_doc/%s", c.config.Addresses[0], index, id)

	body, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.Username != "" && c.config.Password != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("index error: %s", resp.Status)
	}

	return nil
}

// Get retrieves a document
func (c *Client) Get(ctx context.Context, index, id string) (*Document, error) {
	url := fmt.Sprintf("%s/%s/_doc/%s", c.config.Addresses[0], index, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	if c.config.Username != "" && c.config.Password != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("document not found")
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("get error: %s", resp.Status)
	}

	var result struct {
		Source json.RawMessage `json:"_source"`
		ID     string          `json:"_id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &Document{
		ID:     result.ID,
		Source: result.Source,
	}, nil
}

// Search searches for documents
//
//	result, _ := client.Search(ctx, "users", "John")
//	for _, hit := range result.Hits {
//	    fmt.Println(string(hit.Source))
//	}
func (c *Client) Search(ctx context.Context, index, query string) (*SearchResult, error) {
	url := fmt.Sprintf("%s/%s/_search", c.config.Addresses[0], index)

	searchBody := map[string]interface{}{
		"query": map[string]interface{}{
			"match": map[string]interface{}{
				"_all": query,
			},
		},
	}

	body, err := json.Marshal(searchBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.Username != "" && c.config.Password != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID     string          `json:"_id"`
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	docs := make([]Document, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		docs = append(docs, Document{
			ID:     hit.ID,
			Source: hit.Source,
		})
	}

	return &SearchResult{
		Total: result.Hits.Total.Value,
		Hits:  docs,
	}, nil
}

// Delete deletes a document
func (c *Client) Delete(ctx context.Context, index, id string) error {
	url := fmt.Sprintf("%s/%s/_doc/%s", c.config.Addresses[0], index, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	if c.config.Username != "" && c.config.Password != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("document not found")
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("delete error: %s", resp.Status)
	}

	return nil
}

// DeleteIndex deletes an index
func (c *Client) DeleteIndex(ctx context.Context, index string) error {
	url := fmt.Sprintf("%s/%s", c.config.Addresses[0], index)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	if c.config.Username != "" && c.config.Password != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("delete index error: %s", resp.Status)
	}

	return nil
}

// Bulk performs bulk operations
func (c *Client) Bulk(ctx context.Context, operations []BulkOperation) error {
	url := fmt.Sprintf("%s/_bulk", c.config.Addresses[0])

	var body bytes.Buffer
	for _, op := range operations {
		meta, _ := json.Marshal(op.Metadata)
		body.Write(meta)
		body.WriteByte('\n')
		if op.Document != nil {
			doc, _ := json.Marshal(op.Document)
			body.Write(doc)
			body.WriteByte('\n')
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-ndjson")
	if c.config.Username != "" && c.config.Password != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("bulk error: %s", resp.Status)
	}

	return nil
}

// BulkOperation represents a bulk operation
type BulkOperation struct {
	Metadata map[string]string
	Document interface{}
}

// Ping checks if Elasticsearch is reachable
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.Addresses[0], nil)
	if err != nil {
		return err
	}

	if c.config.Username != "" && c.config.Password != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ping error: %s", resp.Status)
	}

	return nil
}
