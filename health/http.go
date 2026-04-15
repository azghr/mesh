package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// HTTPHandler provides HTTP handlers for health checks
type HTTPHandler struct {
	checker *Checker
	service string
	version string
	env     string
}

// NewHTTPHandler creates a new HTTP handler for health checks
func NewHTTPHandler(checker *Checker, service, version, env string) *HTTPHandler {
	return &HTTPHandler{
		checker: checker,
		service: service,
		version: version,
		env:     env,
	}
}

// HandleHealth handles GET /health requests
func (h *HTTPHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	results := h.checker.Check(ctx)

	// Determine overall status
	overallStatus := StatusPass
	statusCode := http.StatusOK
	for _, result := range results {
		if result.Status == StatusFail {
			overallStatus = StatusFail
			statusCode = http.StatusServiceUnavailable
			break
		}
		if result.Status == StatusWarn && overallStatus == StatusPass {
			overallStatus = StatusWarn
		}
	}

	response := map[string]interface{}{
		"status":    overallStatus,
		"service":   h.service,
		"version":   h.version,
		"timestamp": time.Now().Format(time.RFC3339),
		"checks":    results,
	}

	if h.env != "" {
		response["environment"] = h.env
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// HandleReady handles GET /health/ready requests (readiness probe)
func (h *HTTPHandler) HandleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	results := h.checker.Check(ctx)

	// Check if any critical checks failed
	allPassing := true
	for _, result := range results {
		if result.Status == StatusFail {
			allPassing = false
			break
		}
	}

	if !allPassing {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "not_ready",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ready",
	})
}

// HandleLive handles GET /health/live requests (liveness probe)
func (h *HTTPHandler) HandleLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "alive",
	})
}
