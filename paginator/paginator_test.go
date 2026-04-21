package paginator

import (
	"net/http"
	"net/url"
	"testing"
)

func TestFromURL_Defaults(t *testing.T) {
	u, _ := url.Parse("/users")

	params, err := FromURL(u)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if params.Page() != DefaultPage {
		t.Errorf("expected page %d, got %d", DefaultPage, params.Page())
	}

	if params.Limit() != DefaultLimit {
		t.Errorf("expected limit %d, got %d", DefaultLimit, params.Limit())
	}

	if params.Offset() != 0 {
		t.Errorf("expected offset 0, got %d", params.Offset())
	}
}

func TestFromURL_WithCustomDefault(t *testing.T) {
	u, _ := url.Parse("/users")

	params, err := FromURL(u, 50)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if params.Limit() != 50 {
		t.Errorf("expected limit 50, got %d", params.Limit())
	}
}

func TestFromURL_WithPageAndLimit(t *testing.T) {
	u, _ := url.Parse("/users?page=3&per_page=25")

	params, err := FromURL(u)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if params.Page() != 3 {
		t.Errorf("expected page 3, got %d", params.Page())
	}

	if params.Limit() != 25 {
		t.Errorf("expected limit 25, got %d", params.Limit())
	}

	if params.Offset() != 50 {
		t.Errorf("expected offset 50, got %d", params.Offset())
	}
}

func TestFromURL_InvalidPage(t *testing.T) {
	u, _ := url.Parse("/users?page=-1")

	_, err := FromURL(u)

	if err != ErrInvalidPage {
		t.Errorf("expected ErrInvalidPage, got %v", err)
	}
}

func TestFromURL_LimitExceedsMax(t *testing.T) {
	u, _ := url.Parse("/users?per_page=200")

	params, err := FromURL(u)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if params.Limit() != DefaultLimit {
		t.Errorf("expected limit %d (default), got %d", DefaultLimit, params.Limit())
	}
}

func TestFromURL_InvalidPageParameter(t *testing.T) {
	u, _ := url.Parse("/users?page=abc")

	params, err := FromURL(u)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if params.Page() != DefaultPage {
		t.Errorf("expected page %d (default), got %d", DefaultPage, params.Page())
	}
}

func TestFromURL_EmptyQuery(t *testing.T) {
	u, _ := url.Parse("/users")

	params, err := FromURL(u)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if params.Page() != 1 || params.Limit() != 20 {
		t.Errorf("expected defaults, got page=%d limit=%d", params.Page(), params.Limit())
	}
}

func TestFromRequest(t *testing.T) {
	req, _ := http.NewRequest("GET", "/users?page=2&per_page=10", nil)

	params, err := FromRequest(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if params.Page() != 2 {
		t.Errorf("expected page 2, got %d", params.Page())
	}

	if params.Limit() != 10 {
		t.Errorf("expected limit 10, got %d", params.Limit())
	}

	if params.Offset() != 10 {
		t.Errorf("expected offset 10, got %d", params.Offset())
	}
}

func TestNewMeta(t *testing.T) {
	meta := NewMeta(100, 2, 20)

	if meta.Total != 100 {
		t.Errorf("expected total 100, got %d", meta.Total)
	}

	if meta.Page != 2 {
		t.Errorf("expected page 2, got %d", meta.Page)
	}

	if meta.PerPage != 20 {
		t.Errorf("expected per_page 20, got %d", meta.PerPage)
	}

	if meta.TotalPages != 5 {
		t.Errorf("expected total_pages 5, got %d", meta.TotalPages)
	}
}

func TestNewMeta_NotDivisible(t *testing.T) {
	meta := NewMeta(105, 1, 20)

	if meta.TotalPages != 6 {
		t.Errorf("expected total_pages 6, got %d", meta.TotalPages)
	}
}

func TestNewMeta_ZeroPerPage(t *testing.T) {
	meta := NewMeta(100, 1, 0)

	if meta.PerPage != 1 {
		t.Errorf("expected per_page 1, got %d", meta.PerPage)
	}
}

func TestMeta_HasNext(t *testing.T) {
	meta := NewMeta(100, 2, 20)

	if !meta.HasNext() {
		t.Error("expected HasNext() to be true")
	}

	meta = NewMeta(100, 5, 20)
	if meta.HasNext() {
		t.Error("expected HasNext() to be false on last page")
	}
}

func TestMeta_HasPrevious(t *testing.T) {
	meta := NewMeta(100, 2, 20)

	if !meta.HasPrevious() {
		t.Error("expected HasPrevious() to be true")
	}

	meta = NewMeta(100, 1, 20)
	if meta.HasPrevious() {
		t.Error("expected HasPrevious() to be false on first page")
	}
}

func TestParams_PageLimit(t *testing.T) {
	u, _ := url.Parse("/users?page=5&per_page=15")
	params, _ := FromURL(u)

	if params.Page() != 5 {
		t.Errorf("expected Page()=5, got %d", params.Page())
	}

	if params.Limit() != 15 {
		t.Errorf("expected Limit()=15, got %d", params.Limit())
	}

	if params.Offset() != 60 {
		t.Errorf("expected Offset()=60, got %d", params.Offset())
	}
}
