package command

import (
	"os"
	"strings"
	"testing"
)

func TestCommandsDocURL(t *testing.T) {
	orig := os.Getenv("APP_HOST")
	defer func() {
		os.Setenv("APP_HOST", orig)
		UpdateDocURL()
	}()

	os.Setenv("APP_HOST", "http://localhost:9090")
	UpdateDocURL()

	got := Commands["進階應用"]["參考連結"]
	want := "http://localhost:9090/docs"
	if got != want {
		t.Errorf("Commands doc URL = %q; want %q", got, want)
	}
}

func TestStringCommandsContainsHost(t *testing.T) {
	orig := os.Getenv("APP_HOST")
	defer func() {
		os.Setenv("APP_HOST", orig)
		UpdateDocURL()
	}()

	os.Setenv("APP_HOST", "https://alertor.example.com")
	UpdateDocURL()

	output := stringCommands()
	if !strings.Contains(output, "https://alertor.example.com/docs") {
		t.Errorf("stringCommands output did not contain host docs link, got:\n%s", output)
	}
}
