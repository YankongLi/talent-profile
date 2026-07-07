CREATE TABLE email_login_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,
    code_hash TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT email_login_codes_email_not_blank CHECK (btrim(email) <> ''),
    CONSTRAINT email_login_codes_attempts_not_negative CHECK (attempts >= 0)
);

CREATE INDEX email_login_codes_active_idx
    ON email_login_codes (lower(email), created_at DESC)
    WHERE consumed_at IS NULL;

CREATE TABLE user_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    CONSTRAINT user_sessions_token_hash_not_blank CHECK (btrim(token_hash) <> '')
);

CREATE INDEX user_sessions_active_user_idx
    ON user_sessions (user_id, expires_at DESC)
    WHERE revoked_at IS NULL;
