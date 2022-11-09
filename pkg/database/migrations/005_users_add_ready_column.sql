-- +goose Up
ALTER TABLE users ADD COLUMN "ready" BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE users DROP COLUMN "ready";
