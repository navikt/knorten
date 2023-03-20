-- +goose Up
ALTER TABLE "sessions" ADD COLUMN "is_admin" BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE "sessions" DROP COLUMN "is_admin";
