package telegram

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	log "github.com/Ptt-Alertor/logrus"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/julienschmidt/httprouter"
	"github.com/wenchen/ptt-alertor/command"
	"github.com/wenchen/ptt-alertor/myutil"
)

var (
	bot          *tgbotapi.BotAPI
	err          error
	token        = os.Getenv("TELEGRAM_TOKEN")
	host         = os.Getenv("APP_HOST")
	defaultQueue = NewMessageQueue(nil, nil)
)

func init() {
	if token == "" {
		log.Warn("Telegram token not configured; Telegram integration is disabled")
		return
	}
	bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		log.WithError(err).Warn("Telegram Bot Initialize Failed")
		return
	}
	// bot.Debug = true
	log.Info("Telegram Authorized on " + bot.Self.UserName)

	defaultQueue.Start()

	if host != "" {
		webhookConfig := tgbotapi.NewWebhook(host + "/telegram/" + token)
		webhookConfig.MaxConnections = 100
		_, err = bot.SetWebhook(webhookConfig)
		if err != nil {
			log.WithError(err).Warn("Telegram Bot Set Webhook Failed")
		} else {
			log.Info("Telegram Bot Sets Webhook Success")
		}
	}
}

// StartConsumer starts the telegram message queue consumer.
func StartConsumer() {
	defaultQueue.Start()
}

// StopConsumer gracefully stops the telegram message queue consumer.
func StopConsumer() {
	defaultQueue.Stop()
}

// HandleRequest handles request from webhook
func HandleRequest(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if bot == nil {
		http.Error(w, "Telegram bot not configured", http.StatusServiceUnavailable)
		return
	}
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).Error("Telegram Read Request Body Failed")
	}

	var update tgbotapi.Update
	json.Unmarshal(bytes, &update)

	if update.CallbackQuery != nil {
		handleCallbackQuery(update)
		return
	}

	if update.Message != nil {
		if update.Message.IsCommand() {
			handleCommand(update)
			return
		}
		if update.Message.Text != "" {
			handleText(update)
			return
		}
	}
}

func handleCallbackQuery(update tgbotapi.Update) {
	var responseText string
	userID := strconv.Itoa(update.CallbackQuery.From.ID)
	switch update.CallbackQuery.Data {
	case "CANCEL":
		responseText = "取消"
	default:
		responseText = command.HandleCommand(update.CallbackQuery.Data, userID, true)
	}
	SendTextMessage(update.CallbackQuery.Message.Chat.ID, responseText)
}

// help - 所有指令清單
// list - 設定清單
// ranking - 熱門關鍵字、作者、推文數
// add - 新增看板關鍵字、作者、推文數
// del - 刪除看板關鍵字、作者、推文數
// showkeyboard - 顯示快捷小鍵盤
// hidekeyboard - 隱藏快捷小鍵盤
func handleCommand(update tgbotapi.Update) {
	var responseText string
	userID := strconv.Itoa(update.Message.From.ID)
	chatID := update.Message.Chat.ID

	switch update.Message.Command() {
	case "add", "del":
		text := update.Message.Command() + " " + update.Message.CommandArguments()
		responseText = command.HandleCommand(text, userID, true)
	case "start":
		command.HandleTelegramFollow(userID, chatID)
		responseText = "歡迎使用 Ptt Alertor\n輸入「指令」查看相關功能。\n\n觀看Demo:\nhttps://media.giphy.com/media/3ohzdF6vidM6I49lQs/giphy.gif"
	case "help":
		responseText = command.HandleCommand("help", userID, true)
	case "list":
		responseText = command.HandleCommand("list", userID, true)
	case "ranking":
		responseText = command.HandleCommand("ranking", userID, true)
	case "showkeyboard":
		showReplyKeyboard(chatID)
		return
	case "hidekeyboard":
		hideReplyKeyboard(chatID)
		return
	default:
		responseText = "I don't know the command"
	}
	SendTextMessage(chatID, responseText)
}

func handleText(update tgbotapi.Update) {
	var responseText string
	userID := strconv.Itoa(update.Message.From.ID)
	chatID := update.Message.Chat.ID
	text := update.Message.Text
	if match, _ := regexp.MatchString("^(刪除|刪除作者)+\\s.*\\*+", text); match {
		sendConfirmation(chatID, text)
		return
	}
	responseText = command.HandleCommand(text, userID, true)
	SendTextMessage(chatID, responseText)
}

func sendConfirmation(chatID int64, cmd string) {
	if bot == nil {
		return
	}
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("是", cmd),
			tgbotapi.NewInlineKeyboardButtonData("否", "CANCEL"),
		))
	msg := tgbotapi.NewMessage(chatID, "確定"+cmd+"？")
	msg.ReplyMarkup = markup
	_ = sendDirect(chatID, msg)
}

const maxCharacters = 4096

// SendTextMessage sends text message to chatID via Redis queue.
func SendTextMessage(chatID int64, text string) {
	if bot == nil {
		return
	}
	for _, msg := range myutil.SplitTextByLineBreak(text, maxCharacters) {
		sendTextMessage(chatID, msg)
	}
}

func sendTextMessage(chatID int64, text string) {
	if bot == nil {
		return
	}
	if err := defaultQueue.Enqueue(chatID, text); err != nil {
		log.WithError(err).WithField("chatID", chatID).Warn("Telegram Redis Enqueue Failed, fallback to direct send")
		msg := tgbotapi.NewMessage(chatID, text)
		msg.DisableWebPagePreview = true
		_ = sendDirect(chatID, msg)
	}
}

func sendDirect(chatID int64, c tgbotapi.Chattable) error {
	if bot == nil {
		return nil
	}
	lock := getChatLock(chatID)
	lock.Lock()
	defer lock.Unlock()

	limiter := getChatLimiter(chatID)

	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		now := time.Now()
		if now.Before(limiter.blockedUntil) {
			sleepDur := limiter.blockedUntil.Sub(now)
			time.Sleep(sleepDur)
			now = time.Now()
		}

		if elapsed := now.Sub(limiter.lastSent); elapsed < time.Second {
			time.Sleep(time.Second - elapsed)
		}

		_, err := bot.Send(c)
		limiter.lastSent = time.Now()
		if err == nil {
			return nil
		}

		retryAfter := extractRetryAfter(err)
		if retryAfter > 0 {
			limiter.blockedUntil = time.Now().Add(retryAfter + 500*time.Millisecond)
			log.WithFields(log.Fields{
				"chatID":     chatID,
				"retryAfter": retryAfter,
				"attempt":    attempt + 1,
			}).Warn("Telegram 429 on direct send, retrying after cooldown")
			continue
		}

		log.WithError(err).WithField("chatID", chatID).Error("Telegram Send Message Failed")
		return err
	}
	return fmt.Errorf("telegram send retries exhausted")
}

func showReplyKeyboard(chatID int64) {
	if bot == nil {
		return
	}
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("清單"),
			tgbotapi.NewKeyboardButton("推文清單"),
			tgbotapi.NewKeyboardButton("排行"),
			tgbotapi.NewKeyboardButton("指令"),
		))
	msg := tgbotapi.NewMessage(chatID, "顯示小鍵盤")
	msg.ReplyMarkup = keyboard
	_ = sendDirect(chatID, msg)
}

func hideReplyKeyboard(chatID int64) {
	if bot == nil {
		return
	}
	msg := tgbotapi.NewMessage(chatID, "隱藏小鍵盤")
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	_ = sendDirect(chatID, msg)
}
