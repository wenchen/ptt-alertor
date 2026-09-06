-- Ptt-Alertor PostgreSQL Database Initialization Script
-- Usage: psql -U postgres -d ptt_alertor -f scripts/init_postgres.sql

-- 1. 文章資料表 (Articles Table)
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

-- 常用查詢索引 (Indexes)
CREATE INDEX IF NOT EXISTS idx_articles_board ON articles(board);
CREATE INDEX IF NOT EXISTS idx_articles_date ON articles(last_push_date_time);

-- 2. 看板快照資料表 (Boards Table)
CREATE TABLE IF NOT EXISTS boards (
    board VARCHAR(64) PRIMARY KEY,
    articles JSONB
);
