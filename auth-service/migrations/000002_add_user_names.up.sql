ALTER TABLE users
    ADD COLUMN first_name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN last_name VARCHAR(100) NOT NULL DEFAULT '';

-- Existing accounts (username-only): keep username as-is; names stay empty until profile update.
COMMENT ON COLUMN users.first_name IS 'User given name (display)';
COMMENT ON COLUMN users.last_name IS 'User family name (display)';
