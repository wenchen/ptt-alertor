package controllers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/alicebob/miniredis"
)

func TestMain(m *testing.M) {
	s, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer s.Close()

	parts := strings.Split(s.Addr(), ":")
	os.Setenv("REDIS_ENDPOINT", parts[0])
	os.Setenv("REDIS_PORT", parts[1])

	os.Exit(m.Run())
}

func TestDocsTemplateHost(t *testing.T) {
	orig := os.Getenv("APP_HOST")
	defer os.Setenv("APP_HOST", orig)

	testHost := "https://alertor.test.com"
	os.Setenv("APP_HOST", testHost)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/docs", nil)

	Docs(w, r, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("Docs() status = %d, error: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	wantOGURL := `<meta property="og:url" content="https://alertor.test.com/docs"`
	if !strings.Contains(body, wantOGURL) {
		t.Errorf("Docs() body does not contain expected og:url %q, body snippet:\n%s", wantOGURL, body[:min(len(body), 500)])
	}
	wantAlternate := `<link rel="alternate" href="https://alertor.test.com/docs"`
	if !strings.Contains(body, wantAlternate) {
		t.Errorf("Docs() body does not contain expected alternate link %q", wantAlternate)
	}
}

func TestDocsTemplateDefaultHost(t *testing.T) {
	orig := os.Getenv("APP_HOST")
	defer os.Setenv("APP_HOST", orig)

	os.Setenv("APP_HOST", "")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/docs", nil)

	Docs(w, r, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("Docs() status = %d, error: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	wantOGURL := `<meta property="og:url" content="https://pttalertor.dinolai.com/docs"`
	if !strings.Contains(body, wantOGURL) {
		t.Errorf("Docs() body does not contain default og:url %q", wantOGURL)
	}
	wantAlternate := `<link rel="alternate" href="https://pttalertor.dinolai.com/docs"`
	if !strings.Contains(body, wantAlternate) {
		t.Errorf("Docs() body does not contain default alternate link %q", wantAlternate)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestIndexRendersTelegramWithoutLine(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	Index(w, r, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("Index() status = %d, error: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "username: @LetsHackPTTAlertorBot") {
		t.Errorf("Index() body does not contain Telegram bot username, got:\n%s", body[:min(len(body), 500)])
	}
	if strings.Contains(body, `href="/line"`) {
		t.Errorf("Index() body still contains href=\"/line\"")
	}
}

func TestTelegramIndex(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/telegram", nil)

	TelegramIndex(w, r, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("TelegramIndex() status = %d, error: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "Telegram") {
		t.Errorf("TelegramIndex() body does not contain Telegram")
	}
}
