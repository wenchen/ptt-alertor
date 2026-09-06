# DynamoDB 替換開源資料庫評估與遷移分析 (DynamoDB Replacement Analysis)

本文件針對 Ptt-Alertor 專案中的 [`models/article/dynamodb.go`](../models/article/dynamodb.go) 與 [`models/board/dynamodb.go`](../models/board/dynamodb.go) 進行深度分析，評估若將 AWS DynamoDB 替換為開源資料庫（Open-Source Database）時的最佳選型、架構影響及具體實作指南。

---

## 一、現有 DynamoDB 實作與存取模式分析

### 1. `models/article/dynamodb.go`
* **目標資料表**：`articles`
* **主鍵 (Primary Key)**：`Code` (String) —— PTT 文章代碼（如 `M.1610000000.A.123`）。
* **欄位結構**：
  * 基本欄位：`Code`, `ID`, `Title`, `Link`, `Date`, `Author`, `Board`, `PushSum`, `LastPushDateTime` (RFC3339)。
  * 巢狀欄位：`Comments`（推文資料序列化為 **JSON 字串** 存入，讀取時反序列化為 `[]article.Comment`）。
* **存取行為 (Access Pattern)**：
  * `Find(code string, a *Article)`：依主鍵 `Code` 執行 `GetItem`。
  * `Save(a Article) error`：依主鍵 `Code` 執行全筆覆蓋的 `PutItem`。
  * `Delete(code string) error`：依主鍵 `Code` 執行 `DeleteItem`。

### 2. `models/board/dynamodb.go`
* **目標資料表**：`boards`
* **主鍵 (Primary Key)**：`Board` (String) —— 看板名稱（如 `Gossiping`）。
* **欄位結構**：
  * `Board` (主鍵)
  * `Articles` (String) —— 該板目前最新抓取到的文章列表快照，同樣直接序列化為 **JSON 字串** 儲存。
* **存取行為 (Access Pattern)**：
  * `GetArticles(boardName string)`：依看板名 `GetItem` 取得 Articles JSON。
  * `Save(boardName string, articles article.Articles) error`：全筆覆蓋寫入 `PutItem`。
  * `Delete(boardName string) error`：依看板名刪除 `DeleteItem`。

### 3. 現有架構特性總結
1. **純粹的 Key-Value 存取模式**：
   * 專案完全沒有使用 DynamoDB 的進階查詢功能（例如次級索引 GSI/LSI、複合排序鍵 Range Key、條件寫入 ConditionExpression、跨表交易 Transactions 或 Scan 掃描）。
   * 讀寫皆基於單一 Primary Key，並將複雜結構作為 JSON 字串儲存。
2. **已具備良好的抽象介面 (Driver Interface)**：
   * 文章模組：[`models/article/article.go`](../models/article/article.go) 定義了 `Driver` 介面：
     ```go
     type Driver interface {
         Find(code string, article *Article)
         Save(a Article) error
         Delete(code string) error
     }
     ```
   * 看板模組：[`models/board/board.go`](../models/board/board.go) 定義了 `Driver` 介面：
     ```go
     type Driver interface {
         GetArticles(boardName string) article.Articles
         Save(boardName string, articles article.Articles) error
         Delete(boardName string) error
     }
     ```
   * 系統只需在 [`models/models.go`](../models/models.go) 中抽換注入的實作即可完成無痛切換。
3. **專案原先即有 Redis 實作**：
   * 專案內已存在 [`models/article/redis.go`](../models/article/redis.go) 與 [`models/board/redis.go`](../models/board/redis.go)。
   * 當初作者透過 [`jobs/migratedb.go`](../jobs/migratedb.go) 將文章由 Redis 遷往 DynamoDB，主要考量是**避免大量歷史文章長期佔用昂貴的記憶體 (RAM)**。

---

## 二、候選開源資料庫方案評比

| 候選資料庫 | 資料模型 | 優勢 | 劣勢 | 推薦評級 |
| :--- | :--- | :--- | :--- | :---: |
| **PostgreSQL** | 關聯式 + 原生 JSONB | 1. 開源關聯式資料庫事實標準，穩定可靠。<br/>2. **原生支援 `JSONB`**，完全對應 `Comments` 與 `Articles` 欄位。<br/>3. 徹底解決 Redis 記憶體成本問題。<br/>4. 未來極易擴充複雜查詢（如多條件篩選、推文統計、全文檢索）。 | 相比純 Key-Value 需建兩張簡單 Table（但維護成本極低）。 | ⭐️⭐️⭐️⭐️⭐️<br/>**(最佳首選)** |
| **現有 Redis / Apache Kvrocks** | Key-Value / 相容 Redis 協定 | 1. **程式碼現成可用**（已有 `models/article/redis.go`）。<br/>2. 專案本已深度依賴 Redis，無需引進新技術棧。<br/>3. 若採用 **Apache Kvrocks**（基於 RocksDB 的磁碟儲存），可解決 RAM 不足問題且 100% 相容現有代碼。 | 若用原生 Redis，隨文章增多可能耗盡記憶體。 | ⭐️⭐️⭐️⭐️<br/>**(改動最少)** |
| **ScyllaDB (Alternator)** | NoSQL (DynamoDB 相容) | 1. 提供 **Alternator 相容模式**，直接相容 DynamoDB API。<br/>2. **Go 程式碼一行都不用改**，只需將 AWS SDK Endpoint 指向自建實例。 | 分散式架構較重，針對單一小規模專案維運成本稍高。 | ⭐️⭐️⭐️⭐️<br/>**(零代碼改動)** |
| **MongoDB** | 文件型 NoSQL (BSON) | 1. 文件型模型天然貼合 JSON 結構。<br/>2. 原生支援 Go struct 自動映射。 | 小型部署下性價比與生態泛用度不及 PostgreSQL。 | ⭐️⭐️⭐️ |
| **SQLite** | 輕量嵌入式關聯資料庫 | 1. 單一檔案、零維運、支援 JSON1 擴展。<br/>2. 無需獨立執行 DB 服務程序。 | 多容器部署（ECS scale-out）時會有檔案鎖定與共享儲存瓶頸。 | ⭐️⭐️⭐️<br/>(限單機運行) |

---

## 三、方案推薦與分析決策

### 🏆 首選推薦：**PostgreSQL**
* **推薦理由**：
  1. **資料契合度最高**：PostgreSQL 的 `JSONB` 欄位型態可原生儲存 `Comments` 與 `Articles`，既享有 NoSQL 的結構靈活性，又具備關聯式資料庫的穩定性。
  2. **落實持久化與成本平衡**：將資料儲存於磁碟，徹底解決全放在 Redis 的記憶體成本，滿足原本遷移至 DynamoDB 的初衷。
  3. **未來擴展性最強**：
     * 現有 DynamoDB 僅能依文章代碼 `Code` 單筆查詢。
     * 若未來希望支援「查詢某板推文數超過 100 的熱門文章」、「依日期範圍檢索」、「統計推播效益」，PostgreSQL 僅需添加索引與標準 SQL 即可完成，擴展潛力極佳。

### 🥈 備選方案 A（最小開發成本）：**Apache Kvrocks / 沿用 Redis**
* 若追求開發改動成本趨近於零，專案內原本就已寫好 [`models/article/redis.go`](../models/article/redis.go) 與 [`models/board/redis.go`](../models/board/redis.go)。
* 若擔心記憶體暴增，可部署開源的 **Apache Kvrocks**（它使用 RocksDB 將資料持久化在磁碟，同時相容 Redis 協定），程式碼完全不需重寫。

### 🥉 備選方案 B（零代碼改動）：**ScyllaDB (Alternator)**
* 若必須保留目前的 AWS DynamoDB SDK 邏輯，可直接在本機或伺服器啟動開啟 Alternator 模式的 ScyllaDB，僅需調整 AWS SDK 的 Endpoint URL 即可。

---

## 四、PostgreSQL 具體遷移實作範例

### 步驟 1：建立資料庫綱要 (DDL)
```sql
-- 文章資料表
CREATE TABLE IF NOT EXISTS articles (
    code VARCHAR(64) PRIMARY KEY,
    id INT,
    title TEXT,
    link TEXT,
    date VARCHAR(32),
    author VARCHAR(64),
    board VARCHAR(64),
    push_sum INT,
    last_push_date_time TIMESTAMPTZ,
    comments JSONB
);

CREATE INDEX IF NOT EXISTS idx_articles_board ON articles(board);
CREATE INDEX IF NOT EXISTS idx_articles_date ON articles(last_push_date_time);

-- 看板文章快照資料表
CREATE TABLE IF NOT EXISTS boards (
    board VARCHAR(64) PRIMARY KEY,
    articles JSONB
);
```

### 步驟 2：實作 Article Driver (`models/article/postgres.go`)
```go
package article

import (
	"database/sql"
	"encoding/json"
	log "github.com/Ptt-Alertor/logrus"
)

type Postgres struct {
	DB *sql.DB
}

func (p Postgres) Find(code string, a *Article) {
	var commentsJSON []byte
	query := `SELECT id, code, title, link, date, author, board, push_sum, last_push_date_time, comments 
	          FROM articles WHERE code = $1`
	row := p.DB.QueryRow(query, code)
	err := row.Scan(&a.ID, &a.Code, &a.Title, &a.Link, &a.Date, &a.Author, &a.Board, &a.PushSum, &a.LastPushDateTime, &commentsJSON)
	if err != nil {
		if err != sql.ErrNoRows {
			log.WithError(err).Error("Postgres Find Article Failed")
		}
		return
	}
	if len(commentsJSON) > 0 {
		_ = json.Unmarshal(commentsJSON, &a.Comments)
	}
}

func (p Postgres) Save(a Article) error {
	commentsJSON, err := json.Marshal(a.Comments)
	if err != nil {
		return err
	}
	query := `INSERT INTO articles (code, id, title, link, date, author, board, push_sum, last_push_date_time, comments)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	          ON CONFLICT (code) DO UPDATE SET
	            title = EXCLUDED.title,
	            push_sum = EXCLUDED.push_sum,
	            last_push_date_time = EXCLUDED.last_push_date_time,
	            comments = EXCLUDED.comments`
	_, err = p.DB.Exec(query, a.Code, a.ID, a.Title, a.Link, a.Date, a.Author, a.Board, a.PushSum, a.LastPushDateTime, commentsJSON)
	if err != nil {
		log.WithError(err).Error("Postgres Save Article Failed")
	}
	return err
}

func (p Postgres) Delete(code string) error {
	_, err := p.DB.Exec(`DELETE FROM articles WHERE code = $1`, code)
	if err != nil {
		log.WithError(err).Error("Postgres Delete Article Failed")
	}
	return err
}
```

### 步驟 3：實作 Board Driver (`models/board/postgres.go`)
```go
package board

import (
	"database/sql"
	"encoding/json"
	log "github.com/Ptt-Alertor/logrus"
	"github.com/wenchen/ptt-alertor/models/article"
)

type Postgres struct {
	DB *sql.DB
}

func (p Postgres) GetArticles(boardName string) (articles article.Articles) {
	var articlesJSON []byte
	query := `SELECT articles FROM boards WHERE board = $1`
	row := p.DB.QueryRow(query, boardName)
	err := row.Scan(&articlesJSON)
	if err != nil {
		if err != sql.ErrNoRows {
			log.WithError(err).Error("Postgres Find Board Failed")
		}
		return articles
	}
	if len(articlesJSON) > 0 {
		_ = json.Unmarshal(articlesJSON, &articles)
	}
	return articles
}

func (p Postgres) Save(boardName string, articles article.Articles) error {
	articlesJSON, err := json.Marshal(articles)
	if err != nil {
		return err
	}
	query := `INSERT INTO boards (board, articles) VALUES ($1, $2)
	          ON CONFLICT (board) DO UPDATE SET articles = EXCLUDED.articles`
	_, err = p.DB.Exec(query, boardName, articlesJSON)
	if err != nil {
		log.WithError(err).Error("Postgres Save Board Failed")
	}
	return err
}

func (p Postgres) Delete(boardName string) error {
	_, err := p.DB.Exec(`DELETE FROM boards WHERE board = $1`, boardName)
	if err != nil {
		log.WithError(err).Error("Postgres Delete Board Failed")
	}
	return err
}
```

### 步驟 4：替換注入實作 (`models/models.go`)
最後只需要在 [`models/models.go`](../models/models.go) 中切換實作即完成無縫遷移：

```go
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

// 替換為 Postgres Driver
var Article = func() *article.Article {
	return article.NewArticle(article.Postgres{DB: connections.Postgres()})
}

// 替換為 Postgres Driver
var Board = func() *board.Board {
	return board.NewBoard(board.Postgres{DB: connections.Postgres()}, new(board.Redis))
}
```
