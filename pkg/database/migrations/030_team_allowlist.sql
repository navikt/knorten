-- +goose Up
ALTER TABLE teams ADD COLUMN "enable_allowlist" BOOLEAN DEFAULT false;

-- +goose Down
ALTER TABLE teams DROP COLUMN "enable_allowlist";
