// Package response provides standardized HTTP response helpers.
//
// This package provides helpers for building consistent API responses:
// - Generic Response[T] type for type-safe responses
// - Success and error response helpers
// - Pagination metadata support
// - JSON encoding
//
// Example:
//
//	response.Success(w, users)
//	response.SuccessWithMeta(w, users, total, page, perPage)
//	response.Error(w, errors.NotFoundError("user", id))
package response

import (
	"encoding/json"
	"net/http"

	"github.com/azghr/mesh/errors"
)

// Response is a generic API response wrapper.
type Response[T any] struct {
	Data  T     `json:"data,omitempty"`
	Error *Err  `json:"error,omitempty"`
	Meta  *Meta `json:"meta,omitempty"`
}

// Err represents an error in the response.
type Err struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Meta contains pagination metadata.
type Meta struct {
	Total   int `json:"total"`
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

// Option configures a response.
type Option func(*options)

type options struct {
	status  int
	headers http.Header
}

func newOptions() *options {
	return &options{
		status:  http.StatusOK,
		headers: make(http.Header),
	}
}

// WithStatus sets the HTTP status code.
func WithStatus(code int) Option {
	return func(o *options) {
		o.status = code
	}
}

// WithHeader sets a response header.
func WithHeader(key, value string) Option {
	return func(o *options) {
		o.headers.Set(key, value)
	}
}

// Success writes a successful JSON response.
func Success[T any](w http.ResponseWriter, data T, opts ...Option) {
	SuccessWithMeta(w, data, 0, 0, 0, opts...)
}

// SuccessWithMeta writes a successful JSON response with metadata.
func SuccessWithMeta[T any](w http.ResponseWriter, data T, total, page, perPage int, opts ...Option) {
	o := newOptions()
	for _, opt := range opts {
		opt(o)
	}

	resp := Response[T]{
		Data: data,
	}

	if total > 0 {
		resp.Meta = &Meta{
			Total:   total,
			Page:    page,
			PerPage: perPage,
		}
	}

	writeJSON(w, o.status, o.headers, resp)
}

// Created writes a 201 Created response.
func Created[T any](w http.ResponseWriter, data T, opts ...Option) {
	o := newOptions()
	o.status = http.StatusCreated
	for _, opt := range opts {
		opt(o)
	}

	writeJSON(w, o.status, o.headers, Response[T]{Data: data})
}

// NoContent writes a 204 No Content response.
func NoContent(w http.ResponseWriter, opts ...Option) {
	o := newOptions()
	o.status = http.StatusNoContent
	for _, opt := range opts {
		opt(o)
	}

	for k, v := range o.headers {
		w.Header()[k] = v
	}
	w.WriteHeader(o.status)
}

// Error writes an error response.
func Error(w http.ResponseWriter, err error, opts ...Option) {
	appErr, ok := err.(*errors.AppError)
	if !ok {
		Error(w, errors.InternalError(err.Error()), opts...)
		return
	}

	o := newOptions()
	for _, opt := range opts {
		opt(o)
	}

	resp := Response[any]{
		Error: &Err{
			Code:    string(appErr.Type),
			Message: appErr.Message,
		},
	}

	writeJSON(w, appErr.ToHTTPStatus(), o.headers, resp)
}

// ErrorWithCode writes an error response with custom code.
func ErrorWithCode(w http.ResponseWriter, code int, message string, opts ...Option) {
	o := newOptions()
	for _, opt := range opts {
		opt(o)
	}

	resp := Response[any]{
		Error: &Err{
			Code:    http.StatusText(code),
			Message: message,
		},
	}

	writeJSON(w, code, o.headers, resp)
}

// BadRequest writes a 400 Bad Request response.
func BadRequest(w http.ResponseWriter, message string) {
	ErrorWithCode(w, http.StatusBadRequest, message)
}

// Unauthorized writes a 401 Unauthorized response.
func Unauthorized(w http.ResponseWriter, message string) {
	ErrorWithCode(w, http.StatusUnauthorized, message)
}

// Forbidden writes a 403 Forbidden response.
func Forbidden(w http.ResponseWriter, message string) {
	ErrorWithCode(w, http.StatusForbidden, message)
}

// NotFound writes a 404 Not Found response.
func NotFound(w http.ResponseWriter, message string) {
	ErrorWithCode(w, http.StatusNotFound, message)
}

// Conflict writes a 409 Conflict response.
func Conflict(w http.ResponseWriter, message string) {
	ErrorWithCode(w, http.StatusConflict, message)
}

// InternalError writes a 500 Internal Server Error response.
func InternalError(w http.ResponseWriter, message string) {
	ErrorWithCode(w, http.StatusInternalServerError, message)
}

// writeJSON writes a JSON response with headers.
func writeJSON(w http.ResponseWriter, status int, headers http.Header, data any) {
	w.Header().Set("Content-Type", "application/json")
	for k, v := range headers {
		w.Header()[k] = v
	}
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Log error in production
	}
}
