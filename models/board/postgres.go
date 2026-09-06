package board

import (
	"database/sql"
	"encoding/json"

	log "github.com/Ptt-Alertor/logrus"
	"github.com/wenchen/ptt-alertor/connections"
	"github.com/wenchen/ptt-alertor/models/article"
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

func (p Postgres) GetArticles(boardName string) (articles article.Articles) {
	db := p.getDB()
	var articlesJSON []byte
	query := `SELECT articles FROM boards WHERE board = $1`
	row := db.QueryRow(query, boardName)
	err := row.Scan(&articlesJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			log.WithField("board", boardName).Warn("Board Not Found")
		} else {
			log.WithField("runtime", myutil.BasicRuntimeInfo()).WithError(err).Error("Postgres Find Board Failed")
		}
		return articles
	}
	if len(articlesJSON) > 0 && string(articlesJSON) != "null" {
		err = json.Unmarshal(articlesJSON, &articles)
		if err != nil {
			myutil.LogJSONDecode(err, string(articlesJSON))
		}
	}
	return articles
}

func (p Postgres) Save(boardName string, articles article.Articles) error {
	articlesJSON, err := json.Marshal(articles)
	if err != nil {
		myutil.LogJSONEncode(err, articles)
		return err
	}
	db := p.getDB()
	query := `INSERT INTO boards (board, articles) VALUES ($1, $2)
	          ON CONFLICT (board) DO UPDATE SET articles = EXCLUDED.articles`
	_, err = db.Exec(query, boardName, articlesJSON)
	if err != nil {
		log.WithField("runtime", myutil.BasicRuntimeInfo()).WithError(err).Error("Postgres Save Board Failed")
	}
	return err
}

func (p Postgres) Delete(boardName string) error {
	db := p.getDB()
	query := `DELETE FROM boards WHERE board = $1`
	_, err := db.Exec(query, boardName)
	if err != nil {
		log.WithField("runtime", myutil.BasicRuntimeInfo()).WithError(err).Error("Postgres Delete Board Failed")
	}
	return err
}
