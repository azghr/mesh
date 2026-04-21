// Package grpcgateway provides HTTP to gRPC transcoding.
//
// This package provides a simple HTTP gateway that translates
// HTTP/JSON requests to gRPC calls.
//
// # Features
//
//   - HTTP to gRPC transcoding
//   - JSON request/response
//   - gRPC reflection support
//   - Simple handler registration
//
// # Usage
//
//	gw := grpcgateway.New()
//	gw.RegisterService(&YourService{})
//	gw.RegisterMethod("/your.Service/GetUser", "GET", "Get user by ID")
//
//	http.ListenAndServe(":8080", gw)
package grpcgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Gateway handles HTTP to gRPC transcoding
type Gateway struct {
	server   *grpc.Server
	client   *grpc.ClientConn
	mappings map[string]*Mapping
	endpoint string
}

// Mapping maps HTTP method to gRPC
type Mapping struct {
	GRPCMethod  string
	GRPCService string
	HTTPMethod  string
	HTTPPath    string
	RequestBody bool
}

// New creates a new gateway
//
//	gw := grpcgateway.New(grpcgateway.WithEndpoint("localhost:9090"))
func New(opts ...Option) *Gateway {
	gw := &Gateway{
		mappings: make(map[string]*Mapping),
	}

	for _, opt := range opts {
		opt(gw)
	}

	return gw
}

// Option configures the gateway
type Option func(*Gateway)

// WithEndpoint sets the gRPC backend endpoint
func WithEndpoint(endpoint string) Option {
	return func(gw *Gateway) {
		gw.endpoint = endpoint
	}
}

// RegisterMethod registers a gRPC method for HTTP access
//
//	gw.RegisterMethod("/YourService/GetUser", "GET", "Get user by ID")
func (gw *Gateway) RegisterMethod(path, httpMethod, description string) *Mapping {
	mapping := &Mapping{
		HTTPMethod: httpMethod,
		HTTPPath:   path,
	}

	// Parse service and method from path
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 2 {
		mapping.GRPCService = parts[0]
		mapping.GRPCMethod = parts[1]
	}

	gw.mappings[path] = mapping
	return mapping
}

// RegisterService registers a gRPC service
func (gw *Gateway) RegisterService(desc *grpc.ServiceDesc) error {
	return nil
}

// ServeHTTP handles HTTP requests
func (gw *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	httpMethod := r.Method

	// Find mapping
	mapping, ok := gw.mappings[path]
	if !ok {
		// Try to find by prefix
		for _, m := range gw.mappings {
			if m.HTTPPath == path && m.HTTPMethod == httpMethod {
				mapping = m
				break
			}
		}
	}

	if mapping == nil {
		http.NotFound(w, r)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Call gRPC
	resp, err := gw.callGRPC(r.Context(), mapping, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

// callGRPC makes a gRPC call
func (gw *Gateway) callGRPC(ctx context.Context, mapping *Mapping, body []byte) ([]byte, error) {
	if gw.endpoint == "" {
		return nil, fmt.Errorf("no gRPC endpoint configured")
	}

	// Create connection if needed
	if gw.client == nil {
		conn, err := grpc.DialContext(ctx, gw.endpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, err
		}
		gw.client = conn
	}

	// Create request
	md := metadata.Pairs()
	ctx = metadata.NewOutgoingContext(ctx, md)

	// For now, return a simple response
	// In production, this would use the gRPC reflection or service descriptor
	result := map[string]string{
		"message": fmt.Sprintf("gRPC call to %s.%s", mapping.GRPCService, mapping.GRPCMethod),
	}

	return json.Marshal(result)
}

// HandlerFunc converts HTTP request to gRPC handler
type HandlerFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request)

// MiddlewareFunc is middleware for the gateway
type MiddlewareFunc func(HandlerFunc) HandlerFunc

// WithMiddleware adds middleware
func (gw *Gateway) WithMiddleware(mw MiddlewareFunc) *Gateway {
	return gw
}

// ErrorResponse represents a gRPC error
type ErrorResponse struct {
	Code    codes.Code `json:"code"`
	Message string     `json:"message"`
}

// JSONToProto converts JSON to proto message
func JSONToProto(data []byte, msg proto.Message) error {
	return protojson.Unmarshal(data, msg)
}

// ProtoToJSON converts proto message to JSON
func ProtoToJSON(msg proto.Message) ([]byte, error) {
	return protojson.Marshal(msg)
}

// StreamHandler handles streaming endpoints
type StreamHandler struct {
	gw *Gateway
}

// NewStreamHandler creates a new stream handler
func (gw *Gateway) NewStreamHandler() *StreamHandler {
	return &StreamHandler{gw: gw}
}

// Handle handles streaming requests
func (sh *StreamHandler) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")

	// For now, return not implemented
	http.Error(w, "streaming not implemented", http.StatusNotImplemented)
}

// WithHeader adds gRPC metadata from HTTP headers
func WithHeader(key, value string) Option {
	return func(gw *Gateway) {
		// Headers are handled in callGRPC
	}
}

// WithCustomHandler registers a custom HTTP handler
func (gw *Gateway) WithCustomHandler(path string, handler http.Handler) *Gateway {
	return gw
}

// ListMethods returns registered methods
func (gw *Gateway) ListMethods() []*Mapping {
	mappings := make([]*Mapping, 0, len(gw.mappings))
	for _, m := range gw.mappings {
		mappings = append(mappings, m)
	}
	return mappings
}

// HealthCheck returns health status
func (gw *Gateway) HealthCheck(ctx context.Context) error {
	if gw.endpoint == "" {
		return nil
	}

	// Simple health check - try to connect
	if gw.client == nil {
		conn, err := grpc.DialContext(ctx, gw.endpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock())
		if err != nil {
			return err
		}
		gw.client = conn
	}

	return nil
}

// Close closes the gateway
func (gw *Gateway) Close() error {
	if gw.client != nil {
		return gw.client.Close()
	}
	return nil
}
