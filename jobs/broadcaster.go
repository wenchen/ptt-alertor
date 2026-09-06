package jobs

import (
	"errors"

	"github.com/wenchen/ptt-alertor/models"
	"github.com/wenchen/ptt-alertor/models/user"
)

var platforms = map[string]bool{
	"email":     true,
	"messenger": true,
	"telegram":  true,
}

type Broadcaster struct {
	Checker
	Msg string
}

func (bc Broadcaster) String() string {
	return bc.Msg
}

func (bc Broadcaster) Send(plfms []string) error {
	var platformBl = make(map[string]bool)
	for _, plfm := range plfms {
		if _, ok := platforms[plfm]; !ok {
			return errors.New("platform " + plfm + "is not in broadcast list")
		}
		platformBl[plfm] = true
	}

	for _, u := range models.User().All() {
		bc.subType = "broadcast"
		if platformBl["messenger"] {
			go bc.sendMessenger(u)
		}
		if platformBl["telegram"] {
			go bc.sendTelegram(u)
		}
		if platformBl["email"] {
			go bc.sendEmail(u)
		}
	}
	return nil
}

func (bc Broadcaster) sendEmail(u *user.User) {
	bc.Profile.Email = u.Profile.Email
	ckCh <- bc
}

func (bc Broadcaster) sendMessenger(u *user.User) {
	bc.Profile.Messenger = u.Profile.Messenger
	ckCh <- bc
}

func (bc Broadcaster) sendTelegram(u *user.User) {
	bc.Profile.Telegram = u.Profile.Telegram
	bc.Profile.TelegramChat = u.Profile.TelegramChat
	ckCh <- bc
}
