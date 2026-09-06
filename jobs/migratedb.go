package jobs

import (
	"time"

	log "github.com/Ptt-Alertor/logrus"
	"github.com/wenchen/ptt-alertor/models"
	"github.com/wenchen/ptt-alertor/models/article"
	"github.com/wenchen/ptt-alertor/models/board"
)

var redisArticle = article.NewArticle(new(article.Redis))
var redisBoard = board.NewBoard(new(board.Redis), new(board.Redis))

type migrateDB struct {
}

func NewMigrateDB() *migrateDB {
	return &migrateDB{}
}

func (m migrateDB) Run() {
	m.migrateBoards()
}

func (m migrateDB) migrateBoards() {
	for _, boardName := range models.Board().List() {
		log.WithField("board", boardName).Info("Board Migrating")
		m.migrateBoard(boardName)
		time.Sleep(time.Duration(50 * time.Millisecond))
	}
	log.Info("All Board Migrated")
}

func (migrateDB) migrateBoard(boardName string) {
	redisBoard.Name = boardName

	targetBoard := models.Board()
	targetBoard.Name = boardName
	targetBoard.Articles = redisBoard.GetArticles()

	if err := targetBoard.Save(); err != nil {
		log.WithField("board", boardName).Error("Migrate Board Failed")
	}
}

func (m migrateDB) migrateArticles() {
	for _, code := range new(article.Articles).List() {
		log.WithField("code", code).Info("Article Migrating")
		m.migrateArticle(code)
		time.Sleep(time.Duration(250 * time.Millisecond))
	}
	log.Info("All Article Migrated")
}

func (migrateDB) migrateArticle(code string) {
	targetArticle := models.Article()
	a := redisArticle.Find(code)

	targetArticle.ID = a.ID
	targetArticle.Code = a.Code
	targetArticle.Title = a.Title
	targetArticle.Link = a.Link
	targetArticle.Date = a.Date
	targetArticle.Author = a.Author
	targetArticle.Comments = a.Comments
	targetArticle.LastPushDateTime = a.LastPushDateTime
	targetArticle.Board = a.Board
	targetArticle.PushSum = a.PushSum

	if err := targetArticle.Save(); err != nil {
		log.WithField("code", code).Error("Migrate Article Failed")
	}
}

