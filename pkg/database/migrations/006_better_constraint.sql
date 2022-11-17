-- +goose Up
ALTER TABLE chart_team_values
    DROP CONSTRAINT new_value;

ALTER TABLE chart_team_values
    ADD CONSTRAINT new_value UNIQUE (key, chart_type, team);

-- +goose Down
ALTER TABLE chart_team_values
    DROP CONSTRAINT new_value;

ALTER TABLE chart_team_values
    ADD CONSTRAINT new_value UNIQUE (key, value, chart_type, team);