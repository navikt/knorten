-- +goose Up
DELETE FROM chart_team_values WHERE chart_type='jupyterhub' AND "key"='singleuser.serviceAccountName';

-- +goose Down
INSERT INTO chart_team_values ("key","value","team_id","chart_type")
SELECT 'singleuser.serviceAccountName', v.team_id, v.team_id, 'jupyterhub'
FROM 
(SELECT DISTINCT(team_id) FROM chart_team_values WHERE chart_type='jupyterhub') v;
