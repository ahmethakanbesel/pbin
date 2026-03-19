-- +goose Up
-- +goose StatementBegin
ALTER TABLE files ADD COLUMN delete_secret TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite does not support DROP COLUMN on older versions; this migration is intentionally irreversible.
-- To reverse: recreate the table without delete_secret using CREATE TABLE ... AS SELECT.
SELECT 1;
-- +goose StatementEnd
