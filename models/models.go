package models

import (
	"github.com/wenchen/ptt-alertor/connections"
	"github.com/wenchen/ptt-alertor/models/article"
	"github.com/wenchen/ptt-alertor/models/board"
	"github.com/wenchen/ptt-alertor/models/user"
)

var User = func() *user.User {
	return user.NewUser(new(user.Redis))
}
var Article = func() *article.Article {
	return article.NewArticle(article.Postgres{DB: connections.Postgres()})
}
var Board = func() *board.Board {
	return board.NewBoard(board.Postgres{DB: connections.Postgres()}, new(board.Redis))
}

