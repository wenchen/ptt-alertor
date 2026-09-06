package connections

import (
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	log "github.com/Ptt-Alertor/logrus"
	_ "github.com/lib/pq"
)

var (
	db     *sql.DB
	dbOnce sync.Once
)

const schema = `
CREATE TABLE IF NOT EXISTS articles (
    code VARCHAR(64) PRIMARY KEY,
    id INT,
    title TEXT,
    link TEXT,
    date VARCHAR(32),
    author VARCHAR(64),
    board VARCHAR(64),
    push_sum INT,
    last_push_date_time TIMESTAMPTZ,
    comments JSONB
);

CREATE INDEX IF NOT EXISTS idx_articles_board ON articles(board);
CREATE INDEX IF NOT EXISTS idx_articles_date ON articles(last_push_date_time);

CREATE TABLE IF NOT EXISTS boards (
    board VARCHAR(64) PRIMARY KEY,
    articles JSONB
);
`

func getPostgresConnStr() string {
	if connURL := os.Getenv("DATABASE_URL"); connURL != "" {
		return connURL
	}
	if connURL := os.Getenv("POSTGRES_URL"); connURL != "" {
		return connURL
	}
	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("POSTGRES_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("POSTGRES_USER")
	if user == "" {
		user = "postgres"
	}
	password := os.Getenv("POSTGRES_PASSWORD")
	dbname := os.Getenv("POSTGRES_DBNAME")
	if dbname == "" {
		dbname = "ptt_alertor"
	}
	sslmode := os.Getenv("POSTGRES_SSLMODE")
	if sslmode == "" {
		sslmode = "disable"
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, password, dbname, sslmode)
}

// Postgres returns a singleton *sql.DB pool
func Postgres() *sql.DB {
	dbOnce.Do(func() {
		var err error
		db, err = sql.Open("postgres", getPostgresConnStr())
		if err != nil {
			log.WithError(err).Fatal("Failed to open postgres connection")
		}
		db.SetMaxIdleConns(10)
		db.SetMaxOpenConns(50)
		db.SetConnMaxLifetime(time.Hour)
	})
	return db
}

// AutoMigrate creates the necessary tables and indexes if they do not exist
func AutoMigrate(database *sql.DB) error {
	var err error
	for i := 0; i < 5; i++ {
		_, err = database.Exec(schema)
		if err == nil {
			log.Info("Postgres AutoMigrate Succeeded")
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	log.WithError(err).Error("Postgres AutoMigrate Failed")
	return err
}
