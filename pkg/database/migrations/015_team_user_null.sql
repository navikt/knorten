-- +goose Up
ALTER TABLE teams ALTER COLUMN "users" DROP NOT NULL;

-- +goose Down
ALTER TABLE teams ALTER COLUMN "users" SET NOT NULL;
