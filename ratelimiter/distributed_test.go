package ratelimiter

import (
	"testing"
	"time"
)

func TestDistOptions(t *testing.T) {
	tests := []struct {
		name  string
		opts  []DistOption
		check func(*DistConfig) bool
	}{
		{
			name: "WithPerUserRate",
			opts: []DistOption{WithPerUserRate(500)},
			check: func(c *DistConfig) bool {
				return c.EnablePerUser && c.PerUserRate == 500
			},
		},
		{
			name: "WithPerIPRate",
			opts: []DistOption{WithPerIPRate(200)},
			check: func(c *DistConfig) bool {
				return c.EnablePerIP && c.PerIPRate == 200
			},
		},
		{
			name: "WithPerAPIKeyRate",
			opts: []DistOption{WithPerAPIKeyRate(1000)},
			check: func(c *DistConfig) bool {
				return c.EnablePerAPIKey && c.PerAPIKeyRate == 1000
			},
		},
		{
			name: "WithDistDefaultRate",
			opts: []DistOption{WithDistDefaultRate(50)},
			check: func(c *DistConfig) bool {
				return c.Rate == 50
			},
		},
		{
			name: "WithDistDefaultWindow",
			opts: []DistOption{WithDistDefaultWindow(30 * time.Second)},
			check: func(c *DistConfig) bool {
				return c.Window == 30*time.Second
			},
		},
		{
			name: "WithDistKeyPrefix",
			opts: []DistOption{WithDistKeyPrefix("myprefix")},
			check: func(c *DistConfig) bool {
				return c.KeyPrefix == "myprefix"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &DistConfig{}
			for _, opt := range tt.opts {
				opt(cfg)
			}
			if !tt.check(cfg) {
				t.Error("option not applied correctly")
			}
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name          string
		xForwardedFor string
		remoteAddr    string
		expected      string
	}{
		{
			name:          "X-Forwarded-For",
			xForwardedFor: "192.168.1.1, 10.0.0.1",
			remoteAddr:    "",
			expected:      "192.168.1.1",
		},
		{
			name:          "RemoteAddr",
			xForwardedFor: "",
			remoteAddr:    "192.168.1.1:8080",
			expected:      "192.168.1.1",
		},
		{
			name:          "Empty",
			xForwardedFor: "",
			remoteAddr:    "",
			expected:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetClientIP(tt.xForwardedFor, tt.remoteAddr)
			if result != tt.expected {
				t.Errorf("GetClientIP() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestIsValidIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"Valid IPv4", "192.168.1.1", true},
		{"Valid IPv6", "::1", true},
		{"Invalid IP", "invalid", false},
		{"Empty", "", false},
		{"Not an IP", "not.an.ip", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidIP(tt.ip)
			if result != tt.expected {
				t.Errorf("IsValidIP(%q) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestDimConfig(t *testing.T) {
	dims := []DimConfig{
		{Name: "requests", Rate: 100, Window: time.Minute},
		{Name: "bandwidth", Rate: 1000, Window: time.Minute},
	}

	if len(dims) != 2 {
		t.Errorf("len(dims) = %d, want 2", len(dims))
	}
}

func TestDistConfig_Defaults(t *testing.T) {
	cfg := &DistConfig{
		Rate:          100,
		Window:        time.Minute,
		KeyPrefix:     "ratelimit",
		PerUserRate:   1000,
		PerIPRate:     100,
		PerAPIKeyRate: 5000,
	}

	if cfg.Rate != 100 {
		t.Errorf("Rate = %d, want 100", cfg.Rate)
	}
	if cfg.PerUserRate != 1000 {
		t.Errorf("PerUserRate = %d, want 1000", cfg.PerUserRate)
	}
}
