CREATE TABLE IF NOT EXISTS links (
    code TEXT PRIMARY KEY,
    destination TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS clicks (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL REFERENCES links(code) ON DELETE CASCADE,
    clicked_at TIMESTAMPTZ NOT NULL,
    referer TEXT NOT NULL DEFAULT '',
    country TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    ua_family TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS clicks_code_date_idx ON clicks(code, clicked_at);
