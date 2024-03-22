-- +goose Up
INSERT INTO chart_team_values ("key","value","chart_type","team_id")
(SELECT DISTINCT ON (team_id) 'pypiAccess,omit', 'true', chart_type, team_id FROM chart_team_values WHERE chart_type = 'jupyterhub');

-- +goose Down
DELETE FROM chart_team_values WHERE "key" = 'pypiAccess,omit' AND chart_type = 'jupyterhub';
