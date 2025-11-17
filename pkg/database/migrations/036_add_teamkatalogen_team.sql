-- +goose Up
ALTER TABLE teams ADD COLUMN teamkatalogen_team TEXT;
UPDATE teams SET teamkatalogen_team = '' WHERE teamkatalogen_team IS NULL;
ALTER TABLE teams ALTER COLUMN teamkatalogen_team SET NOT NULL;

-- +goose Down
ALTER TABLE teams DROP COLUMN teamkatalogen_team;
