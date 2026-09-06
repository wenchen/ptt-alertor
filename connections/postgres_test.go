package connections

import (
	"os"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGetPostgresConnStr(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("POSTGRES_URL")
		os.Unsetenv("POSTGRES_HOST")
		os.Unsetenv("POSTGRES_PORT")
		os.Unsetenv("POSTGRES_USER")
		os.Unsetenv("POSTGRES_PASSWORD")
		os.Unsetenv("POSTGRES_DBNAME")
		os.Unsetenv("POSTGRES_SSLMODE")

		expected := "host=localhost port=5432 user=postgres password= dbname=ptt_alertor sslmode=disable"
		if got := getPostgresConnStr(); got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("custom env vars", func(t *testing.T) {
		os.Setenv("POSTGRES_HOST", "myhost")
		os.Setenv("POSTGRES_PORT", "5433")
		os.Setenv("POSTGRES_USER", "myuser")
		os.Setenv("POSTGRES_PASSWORD", "mypass")
		os.Setenv("POSTGRES_DBNAME", "mydb")
		os.Setenv("POSTGRES_SSLMODE", "require")
		defer func() {
			os.Unsetenv("POSTGRES_HOST")
			os.Unsetenv("POSTGRES_PORT")
			os.Unsetenv("POSTGRES_USER")
			os.Unsetenv("POSTGRES_PASSWORD")
			os.Unsetenv("POSTGRES_DBNAME")
			os.Unsetenv("POSTGRES_SSLMODE")
		}()

		expected := "host=myhost port=5433 user=myuser password=mypass dbname=mydb sslmode=require"
		if got := getPostgresConnStr(); got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("DATABASE_URL", func(t *testing.T) {
		url := "postgres://user:pass@host:5432/db?sslmode=disable"
		os.Setenv("DATABASE_URL", url)
		defer os.Unsetenv("DATABASE_URL")

		if got := getPostgresConnStr(); got != url {
			t.Errorf("expected %q, got %q", url, got)
		}
	})
}

func TestAutoMigrate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("unexpected error opening sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS articles")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := AutoMigrate(db); err != nil {
		t.Errorf("AutoMigrate() unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
