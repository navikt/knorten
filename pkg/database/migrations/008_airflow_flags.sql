-- +goose Up
ALTER TABLE teams ADD COLUMN "restrict_airflow_egress" BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE teams ADD COLUMN "api_access" BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE teams DROP COLUMN "restrict_airflow_egress";
ALTER TABLE teams DROP COLUMN "api_access";