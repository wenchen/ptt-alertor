# Ptt-Alertor

<img align="right" src="https://raw.githubusercontent.com/wenchen/ptt-alertor/master/logo.jpg">

[![Go Report Card](https://goreportcard.com/badge/github.com/wenchen/ptt-alertor)](https://goreportcard.com/report/github.com/wenchen/ptt-alertor)
[![codecov](https://codecov.io/gh/wenchen/ptt-alertor/branch/master/graph/badge.svg)](https://codecov.io/gh/wenchen/ptt-alertor)
[![Code Climate](https://api.codeclimate.com/v1/badges/f7047295fce56a0465dc/maintainability)](https://codeclimate.com/github/wenchen/ptt-alertor/maintainability)
[![StackShare](https://img.shields.io/badge/tech-stack-0690fa.svg?style=flat)](https://stackshare.io/ptt-alertor/ptt-alertor)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**Ptt-Alertor** 是一個專為 PTT（批踢踢實業坊）設計的開源即時文章與推文通知系統。

使用者可透過 **Telegram Bot**、**Facebook Messenger** 及 **Email (Mailgun)** 即時接收各看板最新發布的文章通知。系統支援彈性的比對規則（關鍵字交集、排除、正則表達式）、作者追蹤、推噓文門檻比對，以及針對特定文章推文動態的即時追蹤。

---

## 核心特色 (Key Features)

* **多元即時通知管道**：支援 Telegram Bot、Facebook Messenger 聊天機器人與 Email 自動推播。
* **豐富訂閱比對規則**：
  * **關鍵字追蹤**：支援多板多關鍵字，並提供 `&`（同時出現）、`!`（排除）、`&!`（排除特定情境）及 `regexp:`（正則表達式）等進階比對。
  * **作者追蹤**：鎖定喜愛作者，第一時間收到最新發文。
  * **推噓文數門檻**：自訂推文或噓文門檻（0–100），掌握爆文或熱門討論。
  * **文章推文追蹤**：追蹤特定文章的後續推噓文與回文動態。
* **現代化雲原生架構**：以 Docker Compose 一鍵編排 Go 核心應用、PostgreSQL 15、Redis 與 Nginx 四大微服務，徹底解除對 AWS 專有雲端服務（DynamoDB、ECS、S3）的依賴。
* **高效能與容錯設計**：
  * **去重比對機制**：利用 Redis 集合運算（`SDIFF`）確保精準推播不重複。
  * **優雅降級 (Graceful Degradation)**：通訊模組金鑰未填時自動關閉該通道，不阻礙 Web 與背景爬蟲服務。
  * **連線重試與自動遷移**：Redis 支援指數退避重試，PostgreSQL 啟動時自動檢查資料庫並執行 `AutoMigrate`。
  * **PTT 存活監控 (PttMonitor)**：定時偵測 PTT 站台狀態，站台維護時自動休眠爬蟲，避免無效請求與 IP 被封鎖。
* **內建診斷與監控**：整合 Google gops agent (`:6060`)，便於即時分析 Goroutine、CPU 及記憶體狀況；首頁提供 WebSocket 即時推播計數器跳動展示。

---

## 系統架構 (System Architecture)

系統由四大容器化微服務構成，僅需 Docker 環境即可完整運行：

```
                +---------------------------------------+
                |    外部通訊平台 / 用戶端 / 瀏覽器     |
                | (Telegram / Messenger / Mailgun / Web)|
                +-------------------+-------------------+
                                    |
                            HTTP (Port 8080)
                            WebSocket (/ws)
                                    v
                +---------------------------------------+
                |           Nginx (nginx:alpine)        |
                |  • 靜態檔案快取託管 (/assets/)        |
                |  • 反向代理轉發動態請求至後端         |
                |  • WebSocket (/ws) 協議升級轉發       |
                +-------------------+-------------------+
                                    | proxy_pass :9090
                                    v
+-----------------------------------------------------------------------+
|                       ptt-alertor (Go 1.27.1)                         |
|  • Web API 伺服器 (:9090)             • PTT 爬蟲與排程 Worker (Jobs)  |
|  • Telegram / Messenger Webhook 處理  • gops 效能診斷代理 (:6060)     |
|  • WebSocket 廣播推播計數             • PTT 存活狀態監控 (PttMonitor) |
+-----------------------+-----------------------+-----------------------+
                        |                       |
               Port 5432|                       |Port 6379
                        v                       v
        +-------------------------------+  +-------------------------------+
        |  PostgreSQL (postgres:15)     |  |       Redis (redis:alpine)    |
        |  • articles (推文 JSONB 儲存) |  |  • 用戶訂閱設定與反向索引     |
        |  • boards (看板文章快照 JSONB)|  |  • SDIFF 推播去重集合運算     |
        |  • 磁碟持久化掛載 (pgdata)    |  |  • 排行榜 (Sorted Set)        |
        |                               |  |  • Pub/Sub 即時推播通知廣播   |
        +-------------------------------+  +-------------------------------+
```

詳細服務架構分析請參閱 [docs/services.md](docs/services.md)。

---

## 部署與快速上手 (Deployment & Quick Start)

### 1. 前置需求 (Prerequisites)

* **Docker & Docker Compose**：推薦使用，一鍵啟動完整微服務架構
* **Go**：1.27.1 或以上版本（本地純 Go 開發時需要）
* **PostgreSQL**：15+（若自行託管資料庫，替代原 DynamoDB）
* **Redis**：6.0+（若自行託管快取伺服器）

---

### 2. 環境變數配置 (Environment Variables)

複製範例設定檔建立 `.env`：

```bash
cp .env.example .env
```

核心配置項目說明：

| 變數名稱 | 預設值 / 範例 | 說明 |
| :--- | :--- | :--- |
| `POSTGRES_HOST` | `postgres` (Docker) / `localhost` | PostgreSQL 伺服器位址 |
| `POSTGRES_PORT` | `5432` | PostgreSQL 連線埠 |
| `POSTGRES_USER` | `postgres` | PostgreSQL 使用者名稱 |
| `POSTGRES_PASSWORD` | `password` | PostgreSQL 密碼 |
| `POSTGRES_DBNAME` | `ptt_alertor` | PostgreSQL 資料庫名稱 |
| `POSTGRES_SSLMODE` | `disable` | SSL 模式（本地設 `disable`，雲端生產環境視需求設定 `require`） |
| `DATABASE_URL` | *(可選)* | 完整 PostgreSQL 連線字串（若設定將優先於上述個別變數） |
| `REDIS_ENDPOINT` | `redis` (Docker) / `localhost` | Redis 主機位址 |
| `REDIS_PORT` | `6379` | Redis 連線埠 |
| `APP_HOST` | `http://localhost` | 應用程式公開對外網址（用於 Webhook、重導向、教學連結） |
| `APP_WS_HOST` | `ws://localhost` | 即時推播 WebSocket 對外連線網址 |
| `HTTP_PORT` | `8080` | Nginx 對外監聽連線埠（預設為 8080，避免與伺服器既有 80 埠衝突） |
| `AUTH_USER` / `AUTH_PW` | `admin` / `password` | 內部管理 API HTTP Basic Auth 帳號與密碼 |
| `BOARD_HIGH` | `gossiping,stock` | 高頻抓取之重點熱門看板（以逗號分隔） |
| `TELEGRAM_TOKEN` | *(選填)* | Telegram 機器人 API Token（未設定時自動關閉該模組） |
| `MESSENGER_ACCESSTOKEN` | *(選填)* | Facebook Messenger 粉絲專頁存取權杖 |
| `MESSENGER_VERIFYTOKEN` | *(選填)* | Facebook Messenger Webhook 驗證 Token |
| `MAILGUN_DOMAIN` | *(選填)* | Mailgun 寄信用網域名稱 |
| `MAILGUN_APIKEY` | *(選填)* | Mailgun 專用 API Key |
| `MAILGUN_PUBLIC_APIKEY` | *(選填)* | Mailgun 公開 API Key |

> **提示（優雅降級）**：通訊軟體各憑證（Telegram、Messenger、Mailgun）皆為選填。未填寫金鑰時，系統會自動在啟動時關閉該通知管道，核心 Web API、後台爬蟲 Worker 與排程不受任何影響。

---

### 3. 一鍵啟動所有服務 (One-Click Start with Docker Compose)

專案已完整配置 `docker-compose.yml`，一行指令即可拉起全套 4 大微服務容器（主程式 `app`、`postgres`、`redis`、`nginx`）：

```bash
docker compose up -d
```

服務啟動後，主要訪問端點如下：

| 端點 / 服務 | 位址 | 說明 |
| :--- | :--- | :--- |
| **網站首頁 & 靜態資源** | `http://localhost:8080` (Port 8080) | 經由 Nginx 反向代理，內建靜態快取與 Gzip 壓縮 |
| **即時推播 WebSocket** | `ws://localhost:8080/ws` | 首頁即時全站推播計數跳動端點 |
| **Go 後端 Web API 直連** | `http://localhost:9090` | Go 核心服務原生 HTTP 監聽端點 |
| **gops 效能診斷代理** | `localhost:6060` | Google gops agent 即時分析 Goroutine、CPU 與記憶體 |

常用 Docker Compose 維運指令：

```bash
# 查看所有容器運作狀態
docker compose ps

# 查看 Go 主程式應用日誌
docker compose logs -f app

# 停止並移除所有容器（資料保留於 pgdata 與 redisdata volume）
docker compose down
```

> **資料庫初始化說明**：PostgreSQL 容器啟動時會自動掛載並執行 `scripts/init_postgres.sql`；主程式連線後亦會自動呼叫 `AutoMigrate` 確保 `articles` 與 `boards` 資料表及索引就緒。

---

### 4. 本地開發模式 (Local Development)

若欲在本地直接透過 `go run` 或進行單元測試開發：

#### 步驟一：僅啟動依賴服務 (PostgreSQL & Redis)

```bash
docker compose up -d postgres redis
```

#### 步驟二：執行單元測試

```bash
# 執行全部測試並檢查競態條件 (Race condition)
go test -race -tags test ./...
```

#### 步驟三：啟動 Go 主程式

```bash
go run main.go
```

---

### 5. 生產環境部署建議 (Production Deployment)

專案已全面轉移至開源自託管的 Docker Compose 微服務架構，徹底擺脫過往 AWS ECS、ECR、SSM 與 DynamoDB 專屬服務綁定，適合部署於任何 Linux VPS（如 DigitalOcean、Linode、Lightsail）或自有實體主機：

1. **伺服器環境安裝**：安裝最新版本的 Docker 與 Docker Compose 插件。
2. **生產環境變數配置**：
   * 建立 `.env` 檔案並填寫生產環境資訊（建議配置高強度資料庫密碼）。
   * 將 `APP_HOST` 設定為正式對外網域（例如 `https://your-domain.com`）。
   * 將 `APP_WS_HOST` 設定為 WebSocket 正式網址（例如 `wss://your-domain.com`）。
3. **HTTPS / SSL 憑證配置**：
   * 可在 Nginx 前層使用 Cloudflare Proxy 提供免費 SSL，或安裝 Certbot (Let's Encrypt) 掛載憑證至 Nginx。
4. **一鍵拉起服務**：
   ```bash
   docker compose up -d --build
   ```
5. **資料持久化與備份**：
   * 資料庫檔案已持久化於 Docker 具名磁區（Volume）`pgdata`。
   * 定期使用 `pg_dump` 建立 PostgreSQL 備份：
     ```bash
     docker compose exec -T postgres pg_dump -U postgres ptt_alertor > ptt_alertor_$(date +%Y%m%d).sql
     ```
   * Redis 資料持久化於 `redisdata` 具名磁區。
6. **線上即時診斷**：
   * 可利用本機 `gops` 工具直接連線至主機的 `:6060` 埠進行 Profiling：
     ```bash
     gops stack localhost:6060   # 輸出所有 Goroutine 堆疊
     gops memstats localhost:6060 # 輸出記憶體分配統計
     ```

---

## 機器人指令與訂閱用法 (Bot Commands & Usage)

使用者可在 Telegram Bot 或 Facebook Messenger 聊天室中輸入以下指令進行訂閱設定：

### 1. 一般指令

| 指令 | 說明 | 範例 |
| :--- | :--- | :--- |
| `清單` / `list` | 查詢目前設定的所有看板、關鍵字、作者與推文 | `清單` |
| `指令` / `help` | 查看可用的指令清單與格式說明 | `指令` |
| `排行` / `ranking` | 查詢全站前五名熱門追蹤關鍵字與作者 | `排行` |

### 2. 關鍵字追蹤

| 動作 | 格式 | 說明與範例 |
| :--- | :--- | :--- |
| **新增** | `新增 [看板] [關鍵字]` | 支援多板與多關鍵字（以逗號分隔）：<br/>`新增 gossiping,movie 金城武,結衣` |
| **刪除** | `刪除 [看板] [關鍵字]` | 取消追蹤指定關鍵字：<br/>`刪除 gossiping 結衣` |

### 3. 作者追蹤

| 動作 | 格式 | 說明與範例 |
| :--- | :--- | :--- |
| **新增作者** | `新增作者 [看板] [作者ID]` | 追蹤指定作者發文：<br/>`新增作者 gossiping ffaarr,obov` |
| **刪除作者** | `刪除作者 [看板] [作者ID]` | 取消追蹤指定作者：<br/>`刪除作者 gossiping obov` |

### 4. 推噓文數門檻追蹤

| 動作 | 格式 | 說明與範例 |
| :--- | :--- | :--- |
| **新增推文數** | `新增推文數 [看板] [門檻]` | 當看板文章推文數達到門檻時發送通知（門檻介於 0–100，0 代表取消）：<br/>`新增推文數 joke,beauty 10` |
| **新增噓文數** | `新增噓文數 [看板] [門檻]` | 當看板文章噓文數達到門檻時發送通知：<br/>`新增噓文數 gossiping 50` |

### 5. 文章推文動態追蹤

| 動作 | 格式 | 說明與範例 |
| :--- | :--- | :--- |
| **新增推文** | `新增推文 [文章網址]` | 追蹤該篇文章的新推文或噓文：<br/>`新增推文 https://www.ptt.cc/bbs/EZsoft/M.1497363598.A.74E.html` |
| **刪除推文** | `刪除推文 [文章網址]` | 取消追蹤該篇文章：<br/>`刪除推文 https://www.ptt.cc/bbs/EZsoft/M.1497363598.A.74E.html` |

### 6. 進階比對語法 (Advanced Matching)

系統支援豐富的比對運算子，可精準過濾文章：

* **同時出現 (`&`)**：標題需同時包含多個關鍵字才會觸發。<br/>`新增 drama-ticket 售&杰倫`（標題同時有「售」和「杰倫」才通知）
* **排除關鍵字 (`!`)**：通知除指定字眼外的所有文章。<br/>`新增 gossiping !問卦`（除了「問卦」以外的文章全部通知）
* **包含 A 並排除 B (`&!`)**：<br/>`新增 gossiping 柯文哲&!問卦`（標題包含「柯文哲」但非「問卦」才通知）
* **正規表示式 (`regexp:`)**：支援完整 Go 正則表達式語法。<br/>`新增 hardwaresale regexp:\[賣/(台中|台北)/.*\]?(RAM|ram)+.*`

### 7. 批量操作 (Batch Operations)

* `**`：代表「所有已設定看板」
* `*`：代表「該板所有關鍵字或作者」

```text
刪除 gossiping *      # 刪除八卦板的所有追蹤關鍵字
刪除 ** 樂透          # 在所有看板中刪除「樂透」關鍵字
刪除 ** *             # 清空所有看板的所有關鍵字設定
刪除作者 gossiping *  # 刪除八卦板所有追蹤作者
刪除作者 ** *         # 清空所有追蹤作者設定
```

---

## HTTP RESTful API 清單 (API Reference)

系統提供 RESTful API 供前端與第三方整合：

### 看板與文章 API (Boards & Articles)

* `GET /boards`：取得所有已抓取之看板列表
* `GET /boards/:boardName/articles`：取得特定看板最新文章快照
* `GET /boards/:boardName/articles/:code`：取得特定文章詳細內容與推文 JSON
* `GET /articles`：取得所有文章資料列表

### 訂閱看板分類 API (Subscription Boards)

* `GET /keyword/boards`：取得有關鍵字訂閱的看板列表
* `GET /author/boards`：取得有作者訂閱的看板列表
* `GET /pushsum/boards`：取得有推噓文數門檻訂閱的看板列表

### 用戶管理 API (User APIs - 需要 HTTP Basic Auth)

管理端點需使用 `AUTH_USER` 與 `AUTH_PW` 進行 HTTP Basic 認證：

* `GET /users`：取得所有使用者清單
* `GET /users/:account`：查詢特定帳號之詳細訂閱條件
* `POST /users`：新增使用者與初始訂閱條件
  ```json
  {
    "profile": {
      "account": "sample_account",
      "email": "sample@mail.com"
    },
    "subscribes": [
      {
        "board": "gossiping",
        "keywords": ["問卦", "爆卦", "公告"]
      },
      {
        "board": "lol",
        "keywords": ["閒聊"]
      }
    ]
  }
  ```
* `PUT /users/:account`：更新特定帳號之訂閱條件
  ```json
  {
    "profile": {
      "account": "sample_account",
      "email": "new_mail@mail.com"
    },
    "subscribes": []
  }
  ```

### 系統廣播與即時通訊 (System Broadcast & Messaging)

* `POST /broadcast` *(需 Basic Auth)*：發布系統廣播訊息給所有註冊用戶
* `GET /ws`：全站推播計數 WebSocket 即時連線端點
* `GET/POST /messenger/webhook`：Facebook Messenger 驗證與訊息接收 Webhook
* `POST /telegram/:token`：Telegram Bot 更新接收 Webhook

### 前端頁面與展示 (Web Pages)

* `GET /`：系統首頁（即時跳動推播計數器、熱門看板、Telegram 與 Messenger 連結）
* `GET /top`：全站熱門排行頁面（熱門關鍵字、熱門作者、推噓文門檻）
* `GET /docs`：指令列與進階比對教學頁面
* `GET /messenger`：Facebook Messenger 綁定教學
* `GET /telegram`：Telegram 機器人使用教學

---

## 架構演進與重大變更 (Architecture Evolution)

| 項目 | 舊版 (Legacy) | 現行版本 (Modernized) | 效益與目的 |
| :--- | :--- | :--- | :--- |
| **持久化資料庫** | AWS DynamoDB | **PostgreSQL 15 (JSONB)** | 解除 AWS 雲端鎖定，改用開源資料庫並以原生 JSONB 儲存推文與快照，降低維運成本。 |
| **靜態資源託管** | Amazon S3 | **Nginx 本地高效託管** | 靜態圖檔與腳本直接透過 Nginx 提供並開啟 Gzip 與快取，移除 S3 依賴。 |
| **容器編排** | AWS ECS / ECR / SSM | **Docker Compose** | 宣告式管理 `app`、`postgres`、`redis`、`nginx` 四大微服務，支援跨平台一鍵自託管。 |
| **LINE 服務** | LINE Bot / LINE Notify | **已全面移除** | 因應外部平台政策調整與簡化系統複雜度，通知重心轉移至 Telegram 與 Messenger。 |
| **語言版本** | Go 1.15 | **Go 1.27.1** | 升級編譯器與相依套件，強化安全性、記憶體回收與執行效率。 |
| **容錯機制** | 缺少金鑰直接 Panic 崩潰 | **優雅降級 (Graceful Degradation)** | 通訊模組金鑰未提供時自動關閉，核心 Web 與爬蟲持續運作；Redis 加入指數退避重試。 |

技術評估與服務詳情：
* [docs/services.md](docs/services.md) - 完整服務架構、容器職責與重構對照清單
* [docs/dynamodb-replacement-analysis.md](docs/dynamodb-replacement-analysis.md) - DynamoDB 遷移至開源 PostgreSQL 之架構評估

---

## Credits

### Real Life

Rose Li, Aries Huang, Scott Kao, Amy Li

### Ptt

DMM, oas, bestpika, Zero0910, lucky0509, wbreeze, chang0206, lindo0130, hungys, gyman7788, tooilxui, myamyakoko, whkuo, papago89, timeline, Kamikiri

### Facebook

Mr.clu, Woqeker

---

## 授權條款 (License)

本專案採用 [Apache License 2.0](LICENSE) 授權開源。