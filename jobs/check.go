package jobs

import (
	log "github.com/Ptt-Alertor/logrus"

	"github.com/wenchen/ptt-alertor/channels/mail"
	"github.com/wenchen/ptt-alertor/channels/messenger"
	"github.com/wenchen/ptt-alertor/channels/telegram"
	"github.com/wenchen/ptt-alertor/models/counter"
)

const workers = 300

var ckCh = make(chan check)

func init() {
	for i := 0; i < workers; i++ {
		go messageWorker(ckCh)
	}
}

func messageWorker(ckCh chan check) {
	for {
		ck := <-ckCh
		sendMessage(ck)
	}
}

type check interface {
	String() string
	Self() Checker
	Stop()
	Run()
}

func sendMessage(c check) {
	cr := c.Self()
	account := cr.Profile.Account
	var platform string
	if cr.Profile.Email != "" {
		platform = "mail"
		sendMail(c)
	}
	if cr.Profile.Messenger != "" {
		platform = "messenger"
		sendMessenger(c)
	}
	if cr.Profile.Telegram != "" {
		platform = "telegram"
		sendTelegram(c)
	}
	counter.IncrAlert()
	log.WithFields(log.Fields{
		"account":  account,
		"platform": platform,
		"board":    cr.board,
		"type":     cr.subType,
		"word":     cr.word,
	}).Info("Message Sent")
}

func sendMail(c check) {
	cr := c.Self()
	m := new(mail.Mail)
	m.Title.BoardName = cr.board
	m.Title.Keyword = cr.keyword
	m.Body.Articles = cr.articles
	m.Receiver = cr.Profile.Email
	m.Send()
}

func sendMessenger(c check) {
	cr := c.Self()
	m := messenger.New()
	m.SendTextMessage(cr.Profile.Messenger, c.String())
}

func sendTelegram(c check) {
	cr := c.Self()
	telegram.SendTextMessage(cr.Profile.TelegramChat, c.String())
}
