package apiversion

import (
	"net/http"
	"testing"
)

func TestCleanVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v1", "v1"},
		{"v2", "v2"},
		{"1", "v1"},
		{"2", "v2"},
		{"", "v"},
		{"  v1  ", "v1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cleanVersion(tt.input)
			if result != tt.expected {
				t.Errorf("cleanVersion(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParsePathVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/v1/users", "v1"},
		{"/v2/users", "v2"},
		{"/api/v1/users", "v1"},
		{"/v10/users", "v10"},
		{"/users", ""},
		{"/vabc", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parsePathVersion(tt.input)
			if result != tt.expected {
				t.Errorf("parsePathVersion(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseAcceptHeader(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"application/vnd.mesh.v1+json", "v1"},
		{"application/vnd.mesh.v2+json", "v2"},
		{"application/json", ""},
		{"text/html", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseAcceptHeader(tt.input)
			if result != tt.expected {
				t.Errorf("parseAcceptHeader(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsLatest(t *testing.T) {
	versions := []string{"v1", "v2", "v3"}

	if !IsLatest("v3", versions) {
		t.Error("expected v3 to be latest")
	}

	if IsLatest("v1", versions) {
		t.Error("expected v1 not to be latest")
	}

	if IsLatest("v99", versions) {
		t.Error("expected v99 not to be latest")
	}

	if !IsLatest("v1", []string{}) {
		t.Error("expected empty versions to return true")
	}

	if !IsLatest("", versions) {
		t.Error("expected empty version to return true")
	}
}

func TestSupportedVersions(t *testing.T) {
	versions := SupportedVersions("v1", "v2", "v3")

	if len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(versions))
	}

	if versions[0] != "v1" {
		t.Errorf("expected v1, got %s", versions[0])
	}
}

func TestFromRequest(t *testing.T) {
	// Test with X-API-Version header
	req, _ := http.NewRequest("GET", "/users", nil)
	req.Header.Set("X-API-Version", "v2")

	version := FromRequest(req, "v1")
	if version != "v2" {
		t.Errorf("expected v2, got %s", version)
	}
}

func TestFromRequest_Default(t *testing.T) {
	req, _ := http.NewRequest("GET", "/users", nil)

	version := FromRequest(req, "v1")
	if version != "v1" {
		t.Errorf("expected v1, got %s", version)
	}
}

func TestFromRequest_AcceptHeader(t *testing.T) {
	req, _ := http.NewRequest("GET", "/users", nil)
	req.Header.Set("Accept", "application/vnd.mesh.v2+json")

	version := FromRequest(req, "v1")
	if version != "v2" {
		t.Errorf("expected v2, got %s", version)
	}
}

func TestFromRequest_PathVersion(t *testing.T) {
	req, _ := http.NewRequest("GET", "/v2/users", nil)

	version := FromRequest(req, "v1")
	if version != "v2" {
		t.Errorf("expected v2, got %s", version)
	}
}
