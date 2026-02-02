CREATE TABLE IF NOT EXISTS users (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  login           VARCHAR(255) NOT NULL,
  password_hash   VARCHAR(255) NOT NULL,
  current_balance NUMERIC(18,2) NOT NULL DEFAULT 0,
  withdrawn_total NUMERIC(18,2) NOT NULL DEFAULT 0,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT users_login_unique UNIQUE (login)
);

CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);

CREATE TABLE IF NOT EXISTS user_sessions (
  id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash   VARCHAR(255) NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at   TIMESTAMPTZ NOT NULL,
  revoked_at   TIMESTAMPTZ NULL,
  last_seen_at TIMESTAMPTZ NULL,
  CONSTRAINT user_sessions_token_unique UNIQUE (token_hash)
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at ON user_sessions(expires_at);

CREATE TABLE IF NOT EXISTS orders (
  id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  number        VARCHAR(50) NOT NULL,
  status        VARCHAR(20) NOT NULL,
  accrual       NUMERIC(18,2) NULL,
  uploaded_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at  TIMESTAMPTZ NULL,
  credited      BOOLEAN NOT NULL DEFAULT FALSE,
  next_poll_at  TIMESTAMPTZ NULL,
  poll_attempts INT NOT NULL DEFAULT 0,
  last_error    VARCHAR(500) NULL,
  CONSTRAINT orders_number_unique UNIQUE (number)
);

CREATE INDEX IF NOT EXISTS idx_orders_user_uploaded_at_desc ON orders(user_id, uploaded_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_status_next_poll ON orders(status, next_poll_at);

CREATE TABLE IF NOT EXISTS withdrawals (
  id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  order_number VARCHAR(50) NOT NULL,
  sum          NUMERIC(18,2) NOT NULL,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_withdrawals_user_processed_at_desc ON withdrawals(user_id, processed_at DESC);
