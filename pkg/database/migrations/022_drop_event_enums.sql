-- +goose Up
DROP TYPE event_status;
DROP TYPE event_type;
DROP TYPE log_type;

-- +goose Down
CREATE TYPE event_status AS ENUM (
    'new',
    'processing',
    'completed',
    'pending',
    'failed'
    );

CREATE TYPE event_type AS ENUM (
    'create:team',
    'update:team',
    'delete:team',
    'create:jupyter',
    'update:jupyter',
    'delete:jupyter',
    'create:airflow',
    'update:airflow',
    'delete:airflow',
    'create:compute',
    'delete:compute'
    );

CREATE TYPE log_type as ENUM (
    'info',
    'error'
    );
