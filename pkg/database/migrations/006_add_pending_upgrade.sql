-- +goose Up
ALTER TABLE teams ADD COLUMN "pending_jupyter_upgrade" BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE teams ADD COLUMN "pending_airflow_upgrade" BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE teams DROP COLUMN "pending_jupyter_upgrade";
ALTER TABLE teams DROP COLUMN "pending_airflow_upgrade";