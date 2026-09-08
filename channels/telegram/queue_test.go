package telegram

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/garyburd/redigo/redis"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type mockSender struct {
	sendFunc func(c tgbotapi.Chattable) (tgbotapi.Message, error)
	sentMsgs []tgbotapi.Chattable
}

func (m *mockSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.sentMsgs = append(m.sentMsgs, c)
	if m.sendFunc != nil {
		return m.sendFunc(c)
	}
	return tgbotapi.Message{}, nil
}

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, func() redis.Conn) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	connFunc := func() redis.Conn {
		conn, err := redis.Dial("tcp", s.Addr())
		if err != nil {
			t.Fatalf("failed to connect to miniredis: %v", err)
		}
		return conn
	}
	return s, connFunc
}

func TestExtractRetryAfter(t *testing.T) {
	// 1. tgbotapi.Error value
	err1 := tgbotapi.Error{
		Message: "Too Many Requests: retry after 8",
		ResponseParameters: tgbotapi.ResponseParameters{
			RetryAfter: 8,
		},
	}
	if dur := extractRetryAfter(err1); dur != 8*time.Second {
		t.Errorf("expected 8s, got %v", dur)
	}

	// 2. tgbotapi.Error pointer
	err2 := &tgbotapi.Error{
		ResponseParameters: tgbotapi.ResponseParameters{
			RetryAfter: 12,
		},
	}
	if dur := extractRetryAfter(err2); dur != 12*time.Second {
		t.Errorf("expected 12s, got %v", dur)
	}

	// 3. String error with regex fallback
	err3 := errors.New(`{"error":"Too Many Requests: retry after 5"}`)
	if dur := extractRetryAfter(err3); dur != 5*time.Second {
		t.Errorf("expected 5s, got %v", dur)
	}

	// 4. Non-429 error
	err4 := errors.New("bad request: chat not found")
	if dur := extractRetryAfter(err4); dur != 0 {
		t.Errorf("expected 0s, got %v", dur)
	}

	// 5. Nil error
	if dur := extractRetryAfter(nil); dur != 0 {
		t.Errorf("expected 0s, got %v", dur)
	}
}

func TestMessageQueue_EnqueueRateLimitPerChat(t *testing.T) {
	s, connFunc := setupTestRedis(t)
	defer s.Close()

	mock := &mockSender{}
	q := NewMessageQueue(connFunc, func() sender { return mock })

	chatID1 := int64(1001)
	chatID2 := int64(1002)

	// Enqueue 3 messages for chatID1
	if err := q.Enqueue(chatID1, "msg 1-1"); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	if err := q.Enqueue(chatID1, "msg 1-2"); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	if err := q.Enqueue(chatID1, "msg 1-3"); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	// Enqueue 1 message for chatID2
	if err := q.Enqueue(chatID2, "msg 2-1"); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	conn := connFunc()
	defer conn.Close()

	// Check next_send for chatID1
	key1 := fmt.Sprintf(ChatNextSendKeyFormat, chatID1)
	val1, err := redis.Int64(conn.Do("GET", key1))
	if err != nil {
		t.Fatalf("failed to get next_send for chat 1: %v", err)
	}

	// Check next_send for chatID2
	key2 := fmt.Sprintf(ChatNextSendKeyFormat, chatID2)
	val2, err := redis.Int64(conn.Do("GET", key2))
	if err != nil {
		t.Fatalf("failed to get next_send for chat 2: %v", err)
	}

	// chatID1 had 3 messages, so next_send should be at least (deliverAt1 + 3000ms)
	// val1 should be greater than val2 by roughly 2000ms
	if val1-val2 < 1900 || val1-val2 > 2100 {
		t.Errorf("expected chat1 next_send to be ~2000ms after chat2, val1=%d, val2=%d, diff=%d", val1, val2, val1-val2)
	}

	// Verify ZSET count
	count, err := redis.Int(conn.Do("ZCARD", ScheduledQueueKey))
	if err != nil {
		t.Fatalf("failed to get zcard: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 items in scheduled queue, got %d", count)
	}
}

func TestMessageQueue_PopReadyAndProcess(t *testing.T) {
	s, connFunc := setupTestRedis(t)
	defer s.Close()

	var sentText string
	mock := &mockSender{
		sendFunc: func(c tgbotapi.Chattable) (tgbotapi.Message, error) {
			if msg, ok := c.(tgbotapi.MessageConfig); ok {
				sentText = msg.Text
			}
			return tgbotapi.Message{}, nil
		},
	}

	q := NewMessageQueue(connFunc, func() sender { return mock })

	chatID := int64(888)
	if err := q.Enqueue(chatID, "hello alertor"); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	nowMs := time.Now().UnixNano() / int64(time.Millisecond)

	// Pop with past time should return nothing
	msgPast, err := q.PopReady(nowMs - 10000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgPast != nil {
		t.Errorf("expected nil for past time, got %+v", msgPast)
	}

	// ProcessOne with future/current time should pop and send
	processed, err := q.ProcessOne(nowMs + 500)
	if err != nil {
		t.Fatalf("process error: %v", err)
	}
	if !processed {
		t.Fatalf("expected message to be processed")
	}

	if sentText != "hello alertor" {
		t.Errorf("expected sentText 'hello alertor', got '%s'", sentText)
	}

	// ZSET should now be empty
	conn := connFunc()
	defer conn.Close()
	count, _ := redis.Int(conn.Do("ZCARD", ScheduledQueueKey))
	if count != 0 {
		t.Errorf("expected 0 items in scheduled queue after processing, got %d", count)
	}
}

func TestMessageQueue_Handle429RetryAfter(t *testing.T) {
	s, connFunc := setupTestRedis(t)
	defer s.Close()

	callCount := 0
	mock := &mockSender{
		sendFunc: func(c tgbotapi.Chattable) (tgbotapi.Message, error) {
			callCount++
			if callCount == 1 {
				// First call fails with 429 Retry After 2s
				return tgbotapi.Message{}, tgbotapi.Error{
					Message: "Too Many Requests: retry after 2",
					ResponseParameters: tgbotapi.ResponseParameters{
						RetryAfter: 2,
					},
				}
			}
			return tgbotapi.Message{}, nil
		},
	}

	q := NewMessageQueue(connFunc, func() sender { return mock })

	chatID := int64(999)
	if err := q.Enqueue(chatID, "retry me"); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	nowMs := time.Now().UnixNano() / int64(time.Millisecond)

	// First attempt: should encounter 429 and reschedule
	processed, err := q.ProcessOne(nowMs + 100)
	if err != nil {
		t.Fatalf("process error: %v", err)
	}
	if !processed {
		t.Fatalf("expected message to be processed on first attempt")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}

	// Verify cooldown key is set in Redis
	conn := connFunc()
	defer conn.Close()
	cdKey := fmt.Sprintf(ChatCooldownKeyFormat, chatID)
	cdVal, err := redis.Int64(conn.Do("GET", cdKey))
	if err != nil || cdVal == 0 {
		t.Fatalf("expected cooldown key to be set, got %v, val: %d", err, cdVal)
	}

	// Verify the message is still in ZSET (rescheduled with cooldown timestamp)
	count, _ := redis.Int(conn.Do("ZCARD", ScheduledQueueKey))
	if count != 1 {
		t.Fatalf("expected 1 item rescheduled in queue, got %d", count)
	}

	// Trying to pop before cooldown should return nil
	msgEarly, _ := q.PopReady(nowMs + 500)
	if msgEarly != nil {
		t.Errorf("expected no message popped before cooldown, got %+v", msgEarly)
	}

	// After cooldown, pop and process again (second attempt should succeed)
	futureMs := cdVal + 100
	processedSecond, err := q.ProcessOne(futureMs)
	if err != nil {
		t.Fatalf("process second attempt error: %v", err)
	}
	if !processedSecond {
		t.Fatalf("expected second attempt to process")
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}

	// Now queue should be empty
	countAfter, _ := redis.Int(conn.Do("ZCARD", ScheduledQueueKey))
	if countAfter != 0 {
		t.Errorf("expected queue to be empty after successful retry, got %d", countAfter)
	}
}

func TestMessageQueue_StartStop(t *testing.T) {
	s, connFunc := setupTestRedis(t)
	defer s.Close()

	sentCh := make(chan string, 1)
	mock := &mockSender{
		sendFunc: func(c tgbotapi.Chattable) (tgbotapi.Message, error) {
			if msg, ok := c.(tgbotapi.MessageConfig); ok {
				sentCh <- msg.Text
			}
			return tgbotapi.Message{}, nil
		},
	}

	q := NewMessageQueue(connFunc, func() sender { return mock })
	q.Start()
	defer q.Stop()

	if err := q.Enqueue(12345, "async test"); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	select {
	case text := <-sentCh:
		if text != "async test" {
			t.Errorf("expected 'async test', got '%s'", text)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for background consumer to process message")
	}
}
