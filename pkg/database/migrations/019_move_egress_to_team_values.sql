-- +goose Up
ALTER TABLE teams DROP COLUMN "restrict_airflow_egress";

-- +goose Down
ALTER TABLE teams ADD COLUMN "restrict_airflow_egress" BOOLEAN NOT NULL DEFAULT false;
