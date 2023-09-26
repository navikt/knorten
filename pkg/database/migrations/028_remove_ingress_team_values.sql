-- +goose Up
DELETE FROM chart_team_values WHERE "key" = 'ingress.web.hosts' AND chart_type = 'airflow';
DELETE FROM chart_team_values WHERE "key" IN ('ingress.hosts','ingress.tls') AND chart_type = 'jupyterhub';

-- +goose Down
INSERT INTO chart_team_values ("key","value","chart_type","team_id")
    (SELECT DISTINCT ON ("team_id") 'ingress.web.hosts', CONCAT('[{"name":"',left("team_id", -5),'","tls":{"enabled":true,"secretName":"airflow-certificate"}}]'), "chart_type", "team_id" FROM chart_team_values WHERE chart_type = 'airflow');

INSERT INTO chart_team_values ("key","value","chart_type","team_id")
    (SELECT DISTINCT ON ("team_id") 'ingress.hosts', CONCAT('["', left("team_id", -5), '.jupyter.knada.io','"]'), "chart_type", "team_id" FROM chart_team_values WHERE chart_type = 'jupyterhub');

INSERT INTO chart_team_values ("key","value","chart_type","team_id")
    (SELECT DISTINCT ON ("team_id") 'ingress.tls', CONCAT('[{"hosts":["', left("team_id", -5), '.jupyter.knada.io','"], "secretName": "jupyterhub-certificate"}]'), "chart_type", "team_id" FROM chart_team_values WHERE chart_type = 'jupyterhub');
