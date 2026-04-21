// Package paginator provides standardized pagination for HTTP list endpoints.
//
// This package helps implement consistent pagination across your API endpoints.
// It parses query parameters, calculates pagination metadata, and provides helpers for
// building paginated responses.
//
// Example:
//
//	params, err := paginator.FromRequest(r, 20)
//	if err != nil {
//	    return errors.BadRequest(err)
//	}
//
//	users, err := db.ListUsers(ctx, params.Offset(), params.Limit())
//	if err != nil {
//	    return err
//	}
//
//	total, err := db.CountUsers(ctx)
//	if err != nil {
//	    return err
//	}
//
//	return response.Success(w, users, paginator.NewMeta(total, params.Page(), params.Limit()))
package paginator

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

// Default values
const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
)

// ErrInvalidPage indicates the page parameter is invalid
var ErrInvalidPage = errors.New("invalid page parameter")

// ErrInvalidLimit indicates the limit parameter is invalid
var ErrInvalidLimit = errors.New("invalid limit parameter")

// Params holds parsed pagination parameters from a request.
type Params struct {
	page   int
	limit  int
	offset int
}

// FromRequest parses pagination parameters from an HTTP request query string.
// Default values are applied when parameters are missing or invalid.
func FromRequest(r *http.Request, defaultLimit ...int) (Params, error) {
	return FromURL(r.URL, defaultLimit...)
}

// FromURL parses pagination parameters from a URL.
// Default values are applied when parameters are missing or invalid.
func FromURL(u *url.URL, defaultLimit ...int) (Params, error) {
	dl := DefaultLimit
	if len(defaultLimit) > 0 && defaultLimit[0] > 0 {
		dl = defaultLimit[0]
	}

	page := parseInt(u.Query().Get("page"), DefaultPage)
	limit := parseInt(u.Query().Get("per_page"), dl)

	if page < 1 {
		return Params{}, ErrInvalidPage
	}

	if limit < 1 || limit > MaxLimit {
		limit = dl
	}

	return Params{
		page:   page,
		limit:  limit,
		offset: (page - 1) * limit,
	}, nil
}

// Offset returns the database offset for the current page.
func (p Params) Offset() int {
	return p.offset
}

// Limit returns the page size limit.
func (p Params) Limit() int {
	return p.limit
}

// Page returns the current page number.
func (p Params) Page() int {
	return p.page
}

// Meta holds pagination metadata for response headers.
type Meta struct {
	Total      int `json:"total"`
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalPages int `json:"total_pages,omitempty"`
}

// NewMeta creates pagination metadata for a response.
// total is the total number of items across all pages.
func NewMeta(total, page, perPage int) Meta {
	if perPage < 1 {
		perPage = 1
	}

	totalPages := total / perPage
	if total%perPage != 0 {
		totalPages++
	}

	return Meta{
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}
}

// HasNext returns true if there is a next page.
func (m Meta) HasNext() bool {
	return m.Page < m.TotalPages
}

// HasPrevious returns true if there is a previous page.
func (m Meta) HasPrevious() bool {
	return m.Page > 1
}

// parseInt parses a string to int, returning default if invalid.
func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return i
}
