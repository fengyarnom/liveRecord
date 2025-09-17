CREATE TABLE IF NOT EXISTS users (
    id             BIGSERIAL PRIMARY KEY,
    username       TEXT NOT NULL UNIQUE,
    email          TEXT UNIQUE,
    password_hash  TEXT NOT NULL,
    role           TEXT NOT NULL DEFAULT 'admin',
    is_active      BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at  TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- demo admin: username 'admin', password 'admin123' (bcrypt)
-- hash generated offline; change asap in production
INSERT INTO users (username, password_hash, role, is_active)
VALUES ('admin', '$2a$10$V3H3t0cSqYq6a3z0rE1g8uZ2JmUtoCpi4DA3F4m0hI2pJfTQjV0t2', 'admin', TRUE)
ON CONFLICT (username) DO NOTHING;
