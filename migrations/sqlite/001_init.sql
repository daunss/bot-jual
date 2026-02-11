-- Users interacting via WhatsApp
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY, -- UUID stored as text
    wa_id TEXT NOT NULL UNIQUE,
    wa_jid TEXT,
    display_name TEXT,
    phone_number TEXT,
    language_preference TEXT DEFAULT 'id-ID',
    timezone TEXT DEFAULT 'Asia/Jakarta',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_wa_id ON users(wa_id);

-- Message log for auditing conversation context
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    direction TEXT NOT NULL CHECK (direction IN ('incoming', 'outgoing')),
    message_type TEXT NOT NULL,
    content TEXT,
    media_url TEXT,
    raw_payload TEXT, -- JSONB stored as TEXT
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_messages_user_id_created_at ON messages(user_id, created_at DESC);

-- Orders (top up, bill payment, etc)
CREATE TABLE IF NOT EXISTS orders (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_ref TEXT NOT NULL,
    product_code TEXT NOT NULL,
    amount BIGINT NOT NULL, -- BIGINT supported
    fee BIGINT DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    metadata TEXT, -- JSONB stored as TEXT
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(order_ref)
);

CREATE INDEX IF NOT EXISTS idx_orders_user_id_status ON orders(user_id, status);

-- Deposit tracking
CREATE TABLE IF NOT EXISTS deposits (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    deposit_ref TEXT NOT NULL,
    method TEXT NOT NULL,
    amount BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    metadata TEXT, -- JSONB stored as TEXT
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(deposit_ref)
);

CREATE INDEX IF NOT EXISTS idx_deposits_user_id_status ON deposits(user_id, status);

-- API keys for Gemini (and other providers, future proof)
CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    value TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 100,
    cooldown_until DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, value)
);

CREATE INDEX IF NOT EXISTS idx_api_keys_provider_priority ON api_keys(provider, priority);

-- Rate limit buckets (per user or global)
CREATE TABLE IF NOT EXISTS rate_limits (
    id TEXT PRIMARY KEY,
    scope TEXT NOT NULL,
    key TEXT NOT NULL,
    counter BIGINT NOT NULL DEFAULT 0,
    reset_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(scope, key)
);
