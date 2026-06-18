-- API Token表
CREATE TABLE IF NOT EXISTS bdopsflow_api_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token_encrypted TEXT NOT NULL,
    token_prefix TEXT NOT NULL,
    last_used_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES bdopsflow_users(id) ON DELETE CASCADE,
    UNIQUE(user_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bdopsflow_api_tokens_user_id ON bdopsflow_api_tokens(user_id);
