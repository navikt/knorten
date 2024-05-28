-- +goose Up
DELETE FROM chart_team_values WHERE "key" = 'hub.config.Authenticator.admin_users';
