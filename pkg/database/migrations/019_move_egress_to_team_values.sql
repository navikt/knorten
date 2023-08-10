-- +goose Up
ALTER TABLE teams DROP COLUMN "restrict_airflow_egress";

-- Set up default values for each Airflow already installed
insert into chart_team_values (team_id, "key", "value", chart_type)
select distinct team_id, 'restrictEgress,omit', 'false', 'airflow'::chart_type from chart_team_values where chart_type = 'airflow';

-- +goose Down
ALTER TABLE teams ADD COLUMN "restrict_airflow_egress" BOOLEAN NOT NULL DEFAULT false;

-- Remove restrictEgress,omit key for each installed Airflow
delete from chart_team_values where "key" = 'restrictEgress,omit' and chart_type = 'airflow';
