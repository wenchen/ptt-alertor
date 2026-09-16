package jobs

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/garyburd/redigo/redis"
	"github.com/wenchen/ptt-alertor/models/article"
	"github.com/wenchen/ptt-alertor/models/board"
	"github.com/wenchen/ptt-alertor/models/user"
)

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

func TestTryLockBoard(t *testing.T) {
	checkingBoardsMu.Lock()
	checkingBoards = make(map[string]bool)
	checkingBoardsMu.Unlock()

	// 1. Initial lock should succeed
	if !tryLockBoard("gossiping") {
		t.Errorf("expected tryLockBoard('gossiping') to return true")
	}

	// 2. Second lock on same board (exact case) should fail
	if tryLockBoard("gossiping") {
		t.Errorf("expected second tryLockBoard('gossiping') to return false")
	}

	// 3. Second lock with different case should also fail (case-insensitive)
	if tryLockBoard("Gossiping") {
		t.Errorf("expected tryLockBoard('Gossiping') to return false")
	}
	if tryLockBoard("GOSSIPING") {
		t.Errorf("expected tryLockBoard('GOSSIPING') to return false")
	}

	// 4. Different board should succeed
	if !tryLockBoard("stock") {
		t.Errorf("expected tryLockBoard('stock') to return true")
	}

	// 5. Unlock gossiping and lock again
	unlockBoard("Gossiping")
	if !tryLockBoard("gossiping") {
		t.Errorf("expected tryLockBoard('gossiping') to return true after unlock")
	}

	// Clean up
	unlockBoard("gossiping")
	unlockBoard("stock")
}

func TestTryLockBoardConcurrent(t *testing.T) {
	checkingBoardsMu.Lock()
	checkingBoards = make(map[string]bool)
	checkingBoardsMu.Unlock()

	const concurrency = 20
	var successCount int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tryLockBoard("lifeismoney") {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if successCount != 1 {
		t.Errorf("expected exactly 1 goroutine to acquire lock, got %d", successCount)
	}

	unlockBoard("lifeismoney")
}

func TestArticleIdent(t *testing.T) {
	tests := []struct {
		name string
		art  article.Article
		want string
	}{
		{
			name: "with ID",
			art:  article.Article{ID: 1773630657, Code: "M.1773630657.A.123", Link: "https://www.ptt.cc/bbs/board/M.1773630657.A.123.html"},
			want: "1773630657",
		},
		{
			name: "without ID, with Code",
			art:  article.Article{Code: "M.1773630657.A.123", Link: "https://www.ptt.cc/bbs/board/M.1773630657.A.123.html"},
			want: "M.1773630657.A.123",
		},
		{
			name: "without ID and Code, with Link",
			art:  article.Article{Link: "https://www.ptt.cc/bbs/board/M.1773630657.A.123.html"},
			want: "https://www.ptt.cc/bbs/board/M.1773630657.A.123.html",
		},
		{
			name: "without ID, Code, Link, with Title",
			art:  article.Article{Title: "[情報] 特價商品"},
			want: "[情報] 特價商品",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := articleIdent(tt.art); got != tt.want {
				t.Errorf("articleIdent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsArticleAlerted(t *testing.T) {
	s, connFunc := setupTestRedis(t)
	defer s.Close()

	origConnFunc := redisConnFunc
	redisConnFunc = connFunc
	defer func() { redisConnFunc = origConnFunc }()

	art := article.Article{
		ID:    1773630657,
		Title: "[情報] 特價商品",
		Link:  "https://www.ptt.cc/bbs/lifeismoney/M.1773630657.A.123.html",
	}

	account1 := "177119976"
	account2 := "999888777"
	boardName := "lifeismoney"

	// 1. First alert check for account1 should return false (not alerted yet, sets lock)
	if isArticleAlerted(account1, boardName, art) {
		t.Errorf("expected first isArticleAlerted for account1 to return false")
	}

	// 2. Second alert check for account1 on same article should return true (deduplicated!)
	if !isArticleAlerted(account1, boardName, art) {
		t.Errorf("expected second isArticleAlerted for account1 to return true (deduplicated)")
	}

	// 3. Different user account for the same article should return false (not alerted to account2 yet)
	if isArticleAlerted(account2, boardName, art) {
		t.Errorf("expected first isArticleAlerted for account2 to return false")
	}

	// 4. Same article on a different board should return false
	if isArticleAlerted(account1, "gossiping", art) {
		t.Errorf("expected isArticleAlerted on gossiping to return false")
	}

	// 5. Different article for account1 should return false
	art2 := article.Article{
		ID:    1773630658,
		Title: "[情報] 另一則特價",
	}
	if isArticleAlerted(account1, boardName, art2) {
		t.Errorf("expected isArticleAlerted for art2 to return false")
	}

	// 6. Verify TTL in Redis is approximately 86400 seconds
	key := alertedKeyPrefix + account1 + ":" + boardName + ":" + strconv.Itoa(art.ID)
	ttl := s.TTL(key)
	if ttl <= 0 || ttl > 24*time.Hour {
		t.Errorf("expected TTL near 24h, got %v", ttl)
	}
}

func TestCheckKeywordDeduplication(t *testing.T) {
	s, connFunc := setupTestRedis(t)
	defer s.Close()

	origConnFunc := redisConnFunc
	redisConnFunc = connFunc
	defer func() { redisConnFunc = origConnFunc }()

	bd := &board.Board{
		Name: "lifeismoney",
		NewArticles: article.Articles{
			{
				ID:    1001,
				Title: "[情報] 優惠活動",
				Link:  "https://www.ptt.cc/bbs/lifeismoney/M.1001.A.123.html",
			},
		},
	}

	ch := make(chan Checker, 10)
	cker := Checker{
		board:   "lifeismoney",
		Profile: user.Profile{Account: "177119976"},
		ch:      ch,
	}

	// First time: article matches and has not been alerted -> should send
	checkKeyword("情報", bd, cker)
	select {
	case received := <-ch:
		if len(received.articles) != 1 || received.articles[0].ID != 1001 {
			t.Errorf("unexpected articles in first checkKeyword: %v", received.articles)
		}
	default:
		t.Fatalf("expected message to be sent on first checkKeyword")
	}

	// Second time: exactly same article and board -> deduplicated, channel should NOT receive
	checkKeyword("情報", bd, cker)
	select {
	case received := <-ch:
		t.Errorf("expected no message on second checkKeyword, got: %v", received)
	default:
		// success: deduplication worked!
	}
}

func TestCheckAuthorDeduplication(t *testing.T) {
	s, connFunc := setupTestRedis(t)
	defer s.Close()

	origConnFunc := redisConnFunc
	redisConnFunc = connFunc
	defer func() { redisConnFunc = origConnFunc }()

	bd := &board.Board{
		Name: "gossiping",
		NewArticles: article.Articles{
			{
				ID:     2001,
				Title:  "[問卦] 測試發文",
				Author: "obov",
				Link:   "https://www.ptt.cc/bbs/gossiping/M.2001.A.123.html",
			},
		},
	}

	ch := make(chan Checker, 10)
	cker := Checker{
		board:   "gossiping",
		Profile: user.Profile{Account: "177119976"},
		ch:      ch,
	}

	// First time: should send
	checkAuthor("obov", bd, cker)
	select {
	case received := <-ch:
		if len(received.articles) != 1 || received.articles[0].ID != 2001 {
			t.Errorf("unexpected articles in first checkAuthor: %v", received.articles)
		}
	default:
		t.Fatalf("expected message to be sent on first checkAuthor")
	}

	// Second time: deduplicated, should not send
	checkAuthor("obov", bd, cker)
	select {
	case received := <-ch:
		t.Errorf("expected no message on second checkAuthor, got: %v", received)
	default:
		// success
	}
}
