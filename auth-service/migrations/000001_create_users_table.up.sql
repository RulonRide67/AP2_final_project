-- Enable UUID generation (gen_random_uuid).
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      VARCHAR(64)  NOT NULL,
    email         VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    is_verified   BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMP    NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    updated_at    TIMESTAMP    NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),

    CONSTRAINT users_username_unique UNIQUE (username),
    CONSTRAINT users_email_unique UNIQUE (email)
);

CREATE INDEX idx_users_is_verified ON users (is_verified) WHERE is_verified = FALSE;

COMMENT ON TABLE users IS 'Application user accounts for the auth service';
COMMENT ON COLUMN users.password_hash IS 'Bcrypt/argon2 hash; never store plaintext passwords';
COMMENT ON COLUMN users.is_verified IS 'True after the user completes email verification';
