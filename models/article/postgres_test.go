package article

import (
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgres_Find(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	p := Postgres{DB: db}

	t.Run("success", func(t *testing.T) {
		fixedTime := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
		commentsJSON := `[{"tag":"推 ","userId":"user1","content":": test comment","dateTime":"2026-09-06T12:00:00Z"}]`

		rows := sqlmock.NewRows([]string{
			"id", "code", "title", "link", "date", "author", "board", "push_sum", "last_push_date_time", "comments",
		}).AddRow(
			12345, "M.12345.A.001", "Test Title", "http://ptt.cc/test", "9/06", "author1", "Gossiping", 10, fixedTime, []byte(commentsJSON),
		)

		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, code, COALESCE(title, ''), COALESCE(link, ''), COALESCE(date, ''), COALESCE(author, ''), COALESCE(board, ''), COALESCE(push_sum, 0), last_push_date_time, comments FROM articles WHERE code = $1")).
			WithArgs("M.12345.A.001").
			WillReturnRows(rows)

		var a Article
		p.Find("M.12345.A.001", &a)

		if a.Code != "M.12345.A.001" {
			t.Errorf("expected code M.12345.A.001, got %s", a.Code)
		}
		if a.Title != "Test Title" {
			t.Errorf("expected title Test Title, got %s", a.Title)
		}
		if a.ID != 12345 {
			t.Errorf("expected ID 12345, got %d", a.ID)
		}
		if a.PushSum != 10 {
			t.Errorf("expected PushSum 10, got %d", a.PushSum)
		}
		if len(a.Comments) != 1 || a.Comments[0].UserID != "user1" {
			t.Errorf("expected 1 comment with user1, got %+v", a.Comments)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %s", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, code, COALESCE(title, '')")).
			WithArgs("not-exist").
			WillReturnError(sql.ErrNoRows)

		var a Article
		p.Find("not-exist", &a)
		if a.Code != "" {
			t.Errorf("expected empty article, got %+v", a)
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

	fixedTime := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	a := Article{
		ID:               12345,
		Code:             "M.12345.A.001",
		Title:            "Test Title",
		Link:             "http://ptt.cc/test",
		Date:             "9/06",
		Author:           "author1",
		Board:            "Gossiping",
		PushSum:          10,
		LastPushDateTime: fixedTime,
		Comments: Comments{
			{Tag: "推 ", UserID: "user1", Content: ": comment", DateTime: fixedTime},
		},
	}

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO articles")).
			WithArgs(
				a.Code, a.ID, a.Title, a.Link, a.Date, a.Author, a.Board, a.PushSum, fixedTime, sqlmock.AnyArg(),
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		if err := p.Save(a); err != nil {
			t.Errorf("Postgres.Save() unexpected error: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %s", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO articles")).
			WithArgs(
				a.Code, a.ID, a.Title, a.Link, a.Date, a.Author, a.Board, a.PushSum, fixedTime, sqlmock.AnyArg(),
			).
			WillReturnError(errors.New("db error"))

		if err := p.Save(a); err == nil {
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
		mock.ExpectExec(regexp.QuoteMeta("DELETE FROM articles WHERE code = $1")).
			WithArgs("M.12345.A.001").
			WillReturnResult(sqlmock.NewResult(1, 1))

		if err := p.Delete("M.12345.A.001"); err != nil {
			t.Errorf("Postgres.Delete() unexpected error: %v", err)
		}
	})
}
