CREATE TABLE IF NOT EXISTS sanbao_video_tasks (
    id BIGSERIAL PRIMARY KEY,
    task_id TEXT NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    model TEXT NOT NULL,
    upstream_model TEXT,
    prompt TEXT,
    status TEXT NOT NULL DEFAULT 'queued',
    ratio TEXT,
    resolution TEXT,
    duration_seconds INTEGER NOT NULL DEFAULT 5,
    video_count INTEGER NOT NULL DEFAULT 1,
    request JSONB NOT NULL DEFAULT '{}'::jsonb,
    response JSONB NOT NULL DEFAULT '{}'::jsonb,
    video_url TEXT,
    download_url TEXT,
    error_message TEXT,
    cost NUMERIC(20, 10) NOT NULL DEFAULT 0,
    refunded_at TIMESTAMPTZ,
    refund_amount NUMERIC(20, 10),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sanbao_video_tasks_user_created ON sanbao_video_tasks(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sanbao_video_tasks_api_key_created ON sanbao_video_tasks(api_key_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sanbao_video_tasks_account_id ON sanbao_video_tasks(account_id);
CREATE INDEX IF NOT EXISTS idx_sanbao_video_tasks_status ON sanbao_video_tasks(status);
