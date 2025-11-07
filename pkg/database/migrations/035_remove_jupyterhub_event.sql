-- +goose Up
ALTER TABLE chart_global_values ALTER COLUMN "chart_type" TYPE text;
ALTER TABLE chart_team_values ALTER COLUMN "chart_type" TYPE text;

DELETE FROM chart_team_values WHERE "chart_type" = 'jupyterhub';
DELETE FROM chart_global_values WHERE "chart_type" = 'jupyterhub';

DROP TYPE CHART_TYPE;
CREATE TYPE CHART_TYPE AS ENUM ('airflow');

ALTER TABLE chart_global_values ALTER COLUMN "chart_type" TYPE CHART_TYPE USING "chart_type"::text::CHART_TYPE;
ALTER TABLE chart_team_values ALTER COLUMN "chart_type" TYPE CHART_TYPE USING "chart_type"::text::CHART_TYPE;

-- +goose Down
ALTER TABLE chart_global_values ALTER COLUMN "chart_type" TYPE text;
ALTER TABLE chart_team_values ALTER COLUMN "chart_type" TYPE text;

DROP TYPE CHART_TYPE;
CREATE TYPE CHART_TYPE AS ENUM ('jupyterhub', 'airflow');

ALTER TABLE chart_global_values ALTER COLUMN "chart_type" TYPE CHART_TYPE USING "chart_type"::text::CHART_TYPE;
ALTER TABLE chart_team_values ALTER COLUMN "chart_type" TYPE CHART_TYPE USING "chart_type"::text::CHART_TYPE;
