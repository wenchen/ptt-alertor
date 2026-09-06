# Ptt-Alertor

<img align="right" src="https://raw.githubusercontent.com/wenchen/ptt-alertor/master/logo.jpg">

[![Build Status](https://github.com/wenchen/ptt-alertor/actions/workflows/main.yml/badge.svg)](https://github.com/wenchen/ptt-alertor/actions/workflows/main.yml)
[![codecov](https://codecov.io/gh/wenchen/ptt-alertor/branch/master/graph/badge.svg)](https://codecov.io/gh/wenchen/ptt-alertor)
[![Go Report Card](https://goreportcard.com/badge/github.com/wenchen/ptt-alertor)](https://goreportcard.com/report/github.com/wenchen/ptt-alertor)
[![Code Climate](https://api.codeclimate.com/v1/badges/f7047295fce56a0465dc/maintainability)](https://codeclimate.com/github/wenchen/ptt-alertor/maintainability)
[![StackShare](https://img.shields.io/badge/tech-stack-0690fa.svg?style=flat)](https://stackshare.io/ptt-alertor/ptt-alertor)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## 部署與快速上手 (Deployment & Quick Start)

### 1. 前置需求 (Prerequisites)

* **Go**: 1.27.1 或以上版本
* **Docker & Docker Compose**: 用於快速運行 PostgreSQL 與 Redis 服務或容器化建置
* **PostgreSQL**: 12+（原 DynamoDB 已於最新版本全數替換為 PostgreSQL）
* **Redis**: 5.0+（用於用戶設定、推播比對與排行榜等快取資料）

---

### 2. 環境變數配置 (Environment Variables)

複製範例設定檔建立 `.env`：

```bash
cp .env.example .env
```

核心配置項說明：

| 變數名稱 | 預設值 / 範例 | 說明 |
| :--- | :--- | :--- |
| `POSTGRES_HOST` | `localhost` | PostgreSQL 伺服器位址 |
| `POSTGRES_PORT` | `5432` | PostgreSQL 連線埠 |
| `POSTGRES_USER` | `postgres` | PostgreSQL 使用者名稱 |
| `POSTGRES_PASSWORD` | `password` | PostgreSQL 密碼 |
| `POSTGRES_DBNAME` | `ptt_alertor` | PostgreSQL 資料庫名稱 |
| `POSTGRES_SSLMODE` | `disable` | SSL 模式（本地設 `disable`，雲端生產環境視需求設定 `require`） |
| `DATABASE_URL` | *(可選)* | 完整 PostgreSQL 連線字串（若設定將優先於上述個別變數） |
| `REDIS_ENDPOINT` | `localhost` | Redis 主機位址 |
| `REDIS_PORT` | `6379` | Redis 連線埠 |
| `APP_HOST` | `http://localhost:9090` | 應用程式公開對外網址（用於 Webhook、重導向、指令教學與網頁網址；預設為 `https://pttalertor.dinolai.com`） |
| `AUTH_USER` / `AUTH_PW` | `admin` / `password` | 內部管理 API HTTP Basic Auth 帳號密碼 |

*(各通訊平台如 Facebook Messenger、Telegram、Mailgun 依需求填寫其 API Token / Secret)*

---

### 3. 一鍵啟動所有服務 (One-Click Start with Docker Compose)

專案已完整配置 `docker-compose.yml`，僅需一行指令即可建置並啟動所有服務（主程式 `ptt-alertor`、PostgreSQL 資料庫、Redis 快取）：

```bash
docker compose up -d
```

服務啟動後，主要服務端點如下：
* Web API 伺服器 & 首頁：`http://localhost:9090`
* 即時推播 WebSocket：`ws://localhost:9090/ws`
* gops 效能診斷代理：`localhost:6060`

檢視服務狀態與日誌：
```bash
# 查看所有容器狀態
docker compose ps

# 查看應用程式日誌
docker compose logs -f app

# 停止並移除所有服務容器
docker compose down
```

> **說明**：
> 1. 啟動順序由 `depends_on` 與健康檢查（Healthcheck）自動管理，確保 PostgreSQL 與 Redis 服務完全就緒後才會啟動主應用程式。
> 2. PostgreSQL 啟動時會自動掛載並執行 [`scripts/init_postgres.sql`](file:///scripts/init_postgres.sql)，主程式啟動時亦具備 `AutoMigrate` 機制自動確保資料表存在。
> 3. 若未設定各通訊平台金鑰（如 Telegram、Facebook Messenger 等），系統會優雅降級並關閉該特定通道，核心 Web API、後台爬蟲 Worker 與排程仍可正常運作。若需啟用通訊平台功能，請參考 `.env.example` 建立 `.env` 檔案。

---

### 4. 本地開發模式 (Local Development)

若欲在本地端直接以 `go run` 進行程式開發，可僅透過 Docker Compose 啟動資料庫與快取：

#### 步驟一：僅啟動依賴服務 (PostgreSQL & Redis)

```bash
docker compose up -d postgres redis
```

#### 步驟二：運行測試

執行單元測試並確認競態條件：

```bash
go test -race -tags test ./...
```

#### 步驟三：啟動主服務

```bash
go run main.go
```

---

### 5. 生產環境部署 (Production Deployment / AWS ECS)

專案已內建完整的 CI/CD 自動化與 AWS ECS 部署設定：

1. **資料庫服務準備**：
   * 在 AWS 上建立 Amazon RDS for PostgreSQL（或自建 PostgreSQL 實例）。
   * 確保安全群組（Security Group）允許 ECS Task 存取 Port 5432。
   * 可手動透過 `psql` 執行 [`scripts/init_postgres.sql`](file:///scripts/init_postgres.sql) 或由服務啟動時自動建立。
2. **參數配置**：
   * 在 AWS Systems Manager (SSM) Parameter Store 或安全環境變數檔存放資料庫連線字串及金鑰。
3. **CI/CD 自動部署 (GitHub Actions)**：
   * 當推送 Tag（如 `v1.0.0`）或發布 GitHub Release 時，[`.github/workflows/main.yml`](file:///.github/workflows/main.yml) 將自動：
     1. 執行單元測試與程式碼覆蓋率分析。
     2. 建置 Docker 映像檔並推送至 Amazon ECR (`ptt-alertor-repo`)。
     3. 渲染 [`.aws/task-definition.json`](file:///.aws/task-definition.json) 並執行 ECS 服務滾動更新。
4. **手動更新部署**：
   * 亦可利用專案內附的 [`ecs-deploy`](file:///ecs-deploy) 腳本更新現有 ECS Service：
     ```bash
     ./ecs-deploy -c [CLUSTER_NAME] -n [SERVICE_NAME] -i [ECR_IMAGE_URI] -r [AWS_REGION]
     ```

---

## API

### Board

* GET /boards

* GET /boards/[board name]/articles

* GET /boards/[board name]/articles/[article code]

### Keyword

* GET /keyword/boards

### Author

* GET /author/boards

### PushSum

* GET /pushsum/boards

### Articles

* GET /articles

### User (Auth)

* GET /users

* GET /users/[account]

* POST /users

```json
{
    "profile":{
        "account": "sample",
        "email":"sample@mail.com"
    },
    "subscribes":[
        {
            "board":"gossiping",
            "keywords":["問卦","爆卦","公告"]
        },
        {
            "board":"lol",
            "keywords":["閒聊"]
        }
    ]
}
```

* PUT /users/[account]

```json
{
    "profile":{
        "account": "sample",
        "email":"sample@mail.com"
    },
    "subscribes":[]
}
```

## Credits

### Real Life

Rose Li, Aries Huang, Scott Kao, Amy Li

### Ptt

DMM, oas, bestpika, Zero0910, lucky0509, wbreeze, chang0206, lindo0130, hungys, gyman7788, tooilxui, myamyakoko, whkuo, papago89, timeline, Kamikiri

### Facebook

Mr.clu, Woqeker