package article

import (
	"database/sql"
	"encoding/json"

	log "github.com/Ptt-Alertor/logrus"
	"github.com/wenchen/ptt-alertor/connections"
	"github.com/wenchen/ptt-alertor/myutil"
)

type Postgres struct {
	DB *sql.DB
}

func (p Postgres) getDB() *sql.DB {
	if p.DB != nil {
		return p.DB
	}
	return connections.Postgres()
}

func (p Postgres) Find(code string, a *Article) {
	db := p.getDB()
	var (
		commentsJSON []byte
		lastPush     sql.NullTime
	)
	query := `SELECT id, code, COALESCE(title, ''), COALESCE(link, ''), COALESCE(date, ''), 
	                 COALESCE(author, ''), COALESCE(board, ''), COALESCE(push_sum, 0), 
	                 last_push_date_time, comments 
	          FROM articles WHERE code = $1`
	row := db.QueryRow(query, code)
	err := row.Scan(&a.ID, &a.Code, &a.Title, &a.Link, &a.Date, &a.Author, &a.Board, &a.PushSum, &lastPush, &commentsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			log.WithField("code", code).Warn("Article Not Found")
		} else {
			log.WithField("runtime", myutil.BasicRuntimeInfo()).WithError(err).Error("Postgres Find Article Failed")
		}
		return
	}
	if lastPush.Valid {
		a.LastPushDateTime = lastPush.Time
	}
	if len(commentsJSON) > 0 && string(commentsJSON) != "null" {
		if err = json.Unmarshal(commentsJSON, &a.Comments); err != nil {
			log.WithFields(log.Fields{
				"code":     code,
				"comments": string(commentsJSON),
			}).Warn("Article Comments Unmarshal Failed")
			myutil.LogJSONDecode(err, string(commentsJSON))
		}
	}
}

func (p Postgres) Save(a Article) error {
	if a.Comments == nil {
		a.Comments = Comments{}
	}
	commentsJSON, err := json.Marshal(a.Comments)
	if err != nil {
		myutil.LogJSONEncode(err, a)
		return err
	}

	var lastPush interface{}
	if !a.LastPushDateTime.IsZero() {
		lastPush = a.LastPushDateTime
	}

	query := `INSERT INTO articles (code, id, title, link, date, author, board, push_sum, last_push_date_time, comments)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	          ON CONFLICT (code) DO UPDATE SET
	            id = EXCLUDED.id,
	            title = EXCLUDED.title,
	            link = EXCLUDED.link,
	            date = EXCLUDED.date,
	            author = EXCLUDED.author,
	            board = EXCLUDED.board,
	            push_sum = EXCLUDED.push_sum,
	            last_push_date_time = EXCLUDED.last_push_date_time,
	            comments = EXCLUDED.comments`

	db := p.getDB()
	_, err = db.Exec(query, a.Code, a.ID, a.Title, a.Link, a.Date, a.Author, a.Board, a.PushSum, lastPush, commentsJSON)
	if err != nil {
		log.WithField("runtime", myutil.BasicRuntimeInfo()).WithError(err).Error("Postgres Save Article Failed")
	}
	return err
}

func (p Postgres) Delete(code string) error {
	db := p.getDB()
	_, err := db.Exec(`DELETE FROM articles WHERE code = $1`, code)
	if err != nil {
		log.WithField("runtime", myutil.BasicRuntimeInfo()).WithError(err).Error("Postgres Delete Article Failed")
	}
	return err
}
