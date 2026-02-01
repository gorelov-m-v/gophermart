CREATE TABLE IF NOT EXISTS users (
  id              BIGSERIAL PRIMARY KEY,
  login           TEXT NOT NULL,
  password_hash   TEXT NOT NULL,
  current_balance NUMERIC(18,2) NOT NULL DEFAULT 0,
  withdrawn_total NUMERIC(18,2) NOT NULL DEFAULT 0,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT users_login_unique UNIQUE (login)
);

CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);

CREATE TABLE IF NOT EXISTS user_sessions (
  id           BIGSERIAL PRIMARY KEY,
  user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash   TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at   TIMESTAMPTZ NOT NULL,
  revoked_at   TIMESTAMPTZ NULL,
  last_seen_at TIMESTAMPTZ NULL,
  CONSTRAINT user_sessions_token_unique UNIQUE (token_hash)
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at ON user_sessions(expires_at);

CREATE TABLE IF NOT EXISTS orders (
  id            BIGSERIAL PRIMARY KEY,
  user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  number        TEXT NOT NULL,
  status        TEXT NOT NULL,
  accrual       NUMERIC(18,2) NULL,
  uploaded_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at  TIMESTAMPTZ NULL,
  credited      BOOLEAN NOT NULL DEFAULT FALSE,
  next_poll_at  TIMESTAMPTZ NULL,
  poll_attempts INT NOT NULL DEFAULT 0,
  last_error    TEXT NULL,
  CONSTRAINT orders_number_unique UNIQUE (number)
);

CREATE INDEX IF NOT EXISTS idx_orders_user_uploaded_at_desc ON orders(user_id, uploaded_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_status_next_poll ON orders(status, next_poll_at);

CREATE TABLE IF NOT EXISTS withdrawals (
  id           BIGSERIAL PRIMARY KEY,
  user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  order_number TEXT NOT NULL,
  sum          NUMERIC(18,2) NOT NULL,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_withdrawals_user_processed_at_desc ON withdrawals(user_id, processed_at DESC);
