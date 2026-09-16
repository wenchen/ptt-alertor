package http

import (
	"os"
	"testing"
)

func TestHttpRequest(t *testing.T) {
	reqURL := "https://www.ptt.cc/atom/lifeismoney.xml"
	req, err := HttpRequest(reqURL)
	if err != nil {
		t.Fatalf("HttpRequest() error = %v", err)
	}

	if req.URL.String() != reqURL {
		t.Errorf("req.URL = %v, want %v", req.URL.String(), reqURL)
	}

	if got := req.Header.Get("User-Agent"); got != defaultUserAgent {
		t.Errorf("req.User-Agent = %v, want %v", got, defaultUserAgent)
	}

	if got := req.Header.Get("Accept"); got == "" {
		t.Error("req.Accept header should not be empty")
	}

	if got := req.Header.Get("Accept-Language"); got == "" {
		t.Error("req.Accept-Language header should not be empty")
	}

	if got := req.Header.Get("Cache-Control"); got == "" {
		t.Error("req.Cache-Control header should not be empty")
	}
}

func TestHttpRequest_CustomUserAgent(t *testing.T) {
	customUA := "Custom-Bot/1.0"
	os.Setenv("USER_AGENT", customUA)
	defer os.Unsetenv("USER_AGENT")

	req, err := HttpRequest("https://www.ptt.cc")
	if err != nil {
		t.Fatalf("HttpRequest() error = %v", err)
	}

	if got := req.Header.Get("User-Agent"); got != customUA {
		t.Errorf("req.User-Agent = %v, want %v", got, customUA)
	}
}
