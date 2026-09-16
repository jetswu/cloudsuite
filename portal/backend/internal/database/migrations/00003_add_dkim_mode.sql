-- Sprint 1.3b: DKIM mode selection (rsa | ed25519 | dual).
-- Existing domains default to 'rsa' (the new portal default); no manual
-- data migration is needed.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE domains
ADD COLUMN dkim_mode VARCHAR(20) NOT NULL DEFAULT 'rsa';
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE domains
ADD CONSTRAINT chk_dkim_mode
CHECK (dkim_mode IN ('rsa', 'ed25519', 'dual'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE domains DROP CONSTRAINT IF EXISTS chk_dkim_mode;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE domains DROP COLUMN IF EXISTS dkim_mode;
-- +goose StatementEnd
