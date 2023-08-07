-- +goose Up
ALTER TABLE teams DROP COLUMN "api_access";

-- +goose Down
ALTER TABLE teams ADD COLUMN "api_access" boolean NOT NULL DEFAULT false;
