package board

import (
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wenchen/ptt-alertor/models/article"
)

func TestPostgres_GetArticles(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	p := Postgres{DB: db}

	t.Run("success", func(t *testing.T) {
		articlesJSON := `[{"ID":12345,"code":"M.12345.A.001","Title":"Test Article","Link":"http://ptt.cc/test"}]`
		rows := sqlmock.NewRows([]string{"articles"}).AddRow([]byte(articlesJSON))

		mock.ExpectQuery(regexp.QuoteMeta("SELECT articles FROM boards WHERE board = $1")).
			WithArgs("Gossiping").
			WillReturnRows(rows)

		arts := p.GetArticles("Gossiping")
		if len(arts) != 1 {
			t.Fatalf("expected 1 article, got %d", len(arts))
		}
		if arts[0].Code != "M.12345.A.001" {
			t.Errorf("expected code M.12345.A.001, got %s", arts[0].Code)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %s", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT articles FROM boards WHERE board = $1")).
			WithArgs("NonExistent").
			WillReturnError(sql.ErrNoRows)

		arts := p.GetArticles("NonExistent")
		if len(arts) != 0 {
			t.Errorf("expected 0 articles, got %d", len(arts))
		}
	})
}

func TestPostgres_Save(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	p := Postgres{DB: db}
	articles := article.Articles{
		{ID: 12345, Code: "M.12345.A.001", Title: "Test Title"},
	}

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO boards (board, articles) VALUES ($1, $2) ON CONFLICT (board) DO UPDATE SET articles = EXCLUDED.articles")).
			WithArgs("Gossiping", sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		if err := p.Save("Gossiping", articles); err != nil {
			t.Errorf("Postgres.Save() unexpected error: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %s", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO boards")).
			WithArgs("Gossiping", sqlmock.AnyArg()).
			WillReturnError(errors.New("save error"))

		if err := p.Save("Gossiping", articles); err == nil {
			t.Error("Postgres.Save() expected error, got nil")
		}
	})
}

func TestPostgres_Delete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	p := Postgres{DB: db}

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("DELETE FROM boards WHERE board = $1")).
			WithArgs("Gossiping").
			WillReturnResult(sqlmock.NewResult(1, 1))

		if err := p.Delete("Gossiping"); err != nil {
			t.Errorf("Postgres.Delete() unexpected error: %v", err)
		}
	})
}
