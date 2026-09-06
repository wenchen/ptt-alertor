package myutil

import (
	"os"
	"testing"
)

func TestAppHost(t *testing.T) {
	orig := os.Getenv("APP_HOST")
	defer os.Setenv("APP_HOST", orig)

	tests := []struct {
		envVal   string
		expected string
	}{
		{"", "https://pttalertor.dinolai.com"},
		{"http://localhost:9090", "http://localhost:9090"},
		{"http://localhost:9090/", "http://localhost:9090"},
		{"https://custom.domain.com", "https://custom.domain.com"},
		{"https://custom.domain.com/", "https://custom.domain.com"},
		{"custom.domain.com", "https://custom.domain.com"},
	}

	for _, tt := range tests {
		os.Setenv("APP_HOST", tt.envVal)
		got := AppHost()
		if got != tt.expected {
			t.Errorf("AppHost() with APP_HOST=%q; got %q, want %q", tt.envVal, got, tt.expected)
		}
	}
}
