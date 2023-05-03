-- +goose Up
ALTER TABLE teams ADD COLUMN "owner" TEXT;
UPDATE teams t SET owner = (SELECT users[1] FROM teams where id = t.id), users = (SELECT users[2:] FROM teams WHERE id = t.id);
ALTER TABLE teams ALTER COLUMN "owner" SET NOT NULL;

-- +goose Down
ALTER TABLE teams DROP COLUMN "owner";
