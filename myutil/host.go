package myutil

import (
	"os"
	"strings"
)

// AppHost returns the public application host configured by APP_HOST,
// defaulting to "https://pttalertor.dinolai.com" if not set.
// It ensures there is no trailing slash and includes a protocol scheme.
func AppHost() string {
	h := strings.TrimSpace(os.Getenv("APP_HOST"))
	if h == "" {
		return "https://pttalertor.dinolai.com"
	}
	h = strings.TrimRight(h, "/")
	if !strings.HasPrefix(h, "http://") && !strings.HasPrefix(h, "https://") {
		h = "https://" + h
	}
	return h
}
