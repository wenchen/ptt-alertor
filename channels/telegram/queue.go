package telegram

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/Ptt-Alertor/logrus"
	"github.com/garyburd/redigo/redis"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/wenchen/ptt-alertor/connections"
)

const (
	// ScheduledQueueKey is the Redis ZSET key for scheduled telegram messages.
	ScheduledQueueKey = "telegram:messages:scheduled"
	// ChatNextSendKeyFormat tracks the next available unix ms timestamp a chat can send a message.
	ChatNextSendKeyFormat = "telegram:chat:%d:next_send"
	// ChatCooldownKeyFormat tracks the cooldown unix ms timestamp when a chat receives a 429.
	ChatCooldownKeyFormat = "telegram:chat:%d:cooldown"
	// DefaultSendIntervalMs enforces at most 1 message per second per chat.
	DefaultSendIntervalMs = 1000
	// MaxRetryAttempts is the maximum retries upon 429 before giving up.
	MaxRetryAttempts = 5
)

var (
	retryRegex   = regexp.MustCompile(`(?i)retry after (\d+)`)
	nonceCounter int64

	chatLocksMu sync.Mutex
	chatLocks   = make(map[int64]*sync.Mutex)

	limitersMu sync.Mutex
	limiters   = make(map[int64]*chatLimiter)
)

type chatLimiter struct {
	mu           sync.Mutex
	lastSent     time.Time
	blockedUntil time.Time
	lastAccess   time.Time
}

// QueueMessage is the payload stored in Redis for scheduled Telegram delivery.
type QueueMessage struct {
	ID        string `json:"id"`
	ChatID    int64  `json:"chat_id"`
	Text      string `json:"text"`
	Attempt   int    `json:"attempt"`
	CreatedAt int64  `json:"created_at"`
}

type sender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

func getChatLock(chatID int64) *sync.Mutex {
	chatLocksMu.Lock()
	defer chatLocksMu.Unlock()
	lock, exists := chatLocks[chatID]
	if !exists {
		lock = &sync.Mutex{}
		chatLocks[chatID] = lock
	}
	return lock
}

func getChatLimiter(chatID int64) *chatLimiter {
	limitersMu.Lock()
	defer limitersMu.Unlock()
	l, exists := limiters[chatID]
	if !exists {
		l = &chatLimiter{}
		limiters[chatID] = l
	}
	l.lastAccess = time.Now()
	return l
}

// extractRetryAfter extracts retry-after duration from tgbotapi.Error or regex fallback.
func extractRetryAfter(err error) time.Duration {
	if err == nil {
		return 0
	}
	if apiErr, ok := err.(tgbotapi.Error); ok && apiErr.RetryAfter > 0 {
		return time.Duration(apiErr.RetryAfter) * time.Second
	}
	if apiErr, ok := err.(*tgbotapi.Error); ok && apiErr.RetryAfter > 0 {
		return time.Duration(apiErr.RetryAfter) * time.Second
	}
	if matches := retryRegex.FindStringSubmatch(err.Error()); len(matches) > 1 {
		if sec, parseErr := strconv.Atoi(matches[1]); parseErr == nil && sec > 0 {
			return time.Duration(sec) * time.Second
		}
	}
	return 0
}

// MessageQueue manages scheduling and dispatching Telegram messages with Redis.
type MessageQueue struct {
	connFunc   func() redis.Conn
	senderFunc func() sender
	stopCh     chan struct{}
	wg         sync.WaitGroup
	startOnce  sync.Once
	stopOnce   sync.Once
	isRunning  int32
}

// NewMessageQueue creates a new MessageQueue instance.
func NewMessageQueue(connFunc func() redis.Conn, senderFunc func() sender) *MessageQueue {
	if connFunc == nil {
		connFunc = connections.Redis
	}
	if senderFunc == nil {
		senderFunc = func() sender { return bot }
	}
	return &MessageQueue{
		connFunc:   connFunc,
		senderFunc: senderFunc,
		stopCh:     make(chan struct{}),
	}
}

// Enqueue schedules a message into Redis ZSET with strict 1 msg/sec interval per chat.
func (q *MessageQueue) Enqueue(chatID int64, text string) error {
	lock := getChatLock(chatID)
	lock.Lock()
	defer lock.Unlock()

	conn := q.connFunc()
	defer conn.Close()

	nowMs := time.Now().UnixNano() / int64(time.Millisecond)

	nextSendKey := fmt.Sprintf(ChatNextSendKeyFormat, chatID)
	nextSendMs, _ := redis.Int64(conn.Do("GET", nextSendKey))

	cooldownMs := q.getCooldown(conn, chatID)
	if cooldownMs > nextSendMs {
		nextSendMs = cooldownMs
	}

	deliverAt := nowMs
	if nextSendMs > nowMs {
		deliverAt = nextSendMs
	}

	nextAllowed := deliverAt + DefaultSendIntervalMs
	if _, err := conn.Do("SET", nextSendKey, nextAllowed, "EX", 3600); err != nil {
		return err
	}

	nonce := atomic.AddInt64(&nonceCounter, 1)
	msg := QueueMessage{
		ID:        fmt.Sprintf("%d-%d-%d", chatID, nowMs, nonce),
		ChatID:    chatID,
		Text:      text,
		Attempt:   0,
		CreatedAt: nowMs,
	}

	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = conn.Do("ZADD", ScheduledQueueKey, deliverAt, raw)
	return err
}

// PopReady retrieves and atomically pops a message whose deliverAt <= nowMs from Redis.
func (q *MessageQueue) PopReady(nowMs int64) (*QueueMessage, error) {
	conn := q.connFunc()
	defer conn.Close()

	values, err := redis.Values(conn.Do("ZRANGEBYSCORE", ScheduledQueueKey, "-inf", nowMs, "LIMIT", 0, 1))
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, nil
	}

	rawBytes, ok := values[0].([]byte)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type in redis zset")
	}

	removed, err := redis.Int(conn.Do("ZREM", ScheduledQueueKey, rawBytes))
	if err != nil {
		return nil, err
	}
	if removed == 0 {
		return nil, nil
	}

	var msg QueueMessage
	if err := json.Unmarshal(rawBytes, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ProcessOne attempts to pop and process one ready message. Returns true if a message was processed.
func (q *MessageQueue) ProcessOne(nowMs int64) (bool, error) {
	msg, err := q.PopReady(nowMs)
	if err != nil || msg == nil {
		return false, err
	}

	q.handleMessage(msg, nowMs)
	return true, nil
}

func (q *MessageQueue) handleMessage(msg *QueueMessage, nowMs int64) {
	lock := getChatLock(msg.ChatID)
	lock.Lock()
	defer lock.Unlock()

	conn := q.connFunc()
	defer conn.Close()

	// 1. Check if chat is still in 429 cooldown
	cooldownMs := q.getCooldown(conn, msg.ChatID)
	if cooldownMs > nowMs {
		deliverAt := cooldownMs + DefaultSendIntervalMs
		if err := q.schedule(conn, msg, deliverAt); err != nil {
			log.WithError(err).WithField("chatID", msg.ChatID).Error("Failed to reschedule cooldown message")
		}
		return
	}

	// 2. Enforce 1 second gap in-memory
	limiter := getChatLimiter(msg.ChatID)
	if elapsed := time.Since(limiter.lastSent); elapsed < time.Second {
		time.Sleep(time.Second - elapsed)
	}

	s := q.senderFunc()
	if s == nil {
		log.Warn("Telegram sender is nil, message discarded")
		return
	}

	tgMsg := tgbotapi.NewMessage(msg.ChatID, msg.Text)
	tgMsg.DisableWebPagePreview = true
	_, err := s.Send(tgMsg)
	limiter.lastSent = time.Now()

	if err == nil {
		return
	}

	// 3. Check for 429 Too Many Requests
	retryAfter := extractRetryAfter(err)
	if retryAfter > 0 {
		msg.Attempt++
		if msg.Attempt < MaxRetryAttempts {
			cdMs := time.Now().Add(retryAfter + 500*time.Millisecond).UnixNano() / int64(time.Millisecond)
			q.setCooldown(conn, msg.ChatID, cdMs, int(retryAfter.Seconds())+60)
			if err := q.schedule(conn, msg, cdMs); err != nil {
				log.WithError(err).WithField("chatID", msg.ChatID).Error("Failed to reschedule 429 message")
			}
			log.WithFields(log.Fields{
				"chatID":     msg.ChatID,
				"retryAfter": retryAfter,
				"attempt":    msg.Attempt,
			}).Warn("Telegram 429 Too Many Requests, rescheduled message")
			return
		}
		log.WithFields(log.Fields{
			"chatID":  msg.ChatID,
			"attempt": msg.Attempt,
		}).Error("Telegram Send Message Failed: max retries reached after 429")
		return
	}

	log.WithError(err).WithField("chatID", msg.ChatID).Error("Telegram Send Message Failed")
}

func (q *MessageQueue) schedule(conn redis.Conn, msg *QueueMessage, deliverAt int64) error {
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	nextSendKey := fmt.Sprintf(ChatNextSendKeyFormat, msg.ChatID)
	nextAllowed := deliverAt + DefaultSendIntervalMs
	curNext, _ := redis.Int64(conn.Do("GET", nextSendKey))
	if nextAllowed > curNext {
		_, _ = conn.Do("SET", nextSendKey, nextAllowed, "EX", 3600)
	}

	_, err = conn.Do("ZADD", ScheduledQueueKey, deliverAt, raw)
	return err
}

func (q *MessageQueue) getCooldown(conn redis.Conn, chatID int64) int64 {
	val, _ := redis.Int64(conn.Do("GET", fmt.Sprintf(ChatCooldownKeyFormat, chatID)))
	return val
}

func (q *MessageQueue) setCooldown(conn redis.Conn, chatID int64, cooldownMs int64, ttlSec int) {
	_, _ = conn.Do("SET", fmt.Sprintf(ChatCooldownKeyFormat, chatID), cooldownMs, "EX", ttlSec)
}

// Start launches the consumer worker in a background goroutine.
func (q *MessageQueue) Start() {
	q.startOnce.Do(func() {
		atomic.StoreInt32(&q.isRunning, 1)
		q.wg.Add(1)
		go func() {
			defer q.wg.Done()
			for atomic.LoadInt32(&q.isRunning) == 1 {
				select {
				case <-q.stopCh:
					return
				default:
				}

				nowMs := time.Now().UnixNano() / int64(time.Millisecond)
				processed, err := q.ProcessOne(nowMs)
				if err != nil {
					log.WithError(err).Warn("Telegram queue process error")
					time.Sleep(100 * time.Millisecond)
					continue
				}
				if !processed {
					time.Sleep(100 * time.Millisecond)
				}
			}
		}()
	})
}

// Stop stops the consumer worker gracefully.
func (q *MessageQueue) Stop() {
	q.stopOnce.Do(func() {
		atomic.StoreInt32(&q.isRunning, 0)
		close(q.stopCh)
		q.wg.Wait()
	})
}
