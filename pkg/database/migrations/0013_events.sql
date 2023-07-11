-- +goose Up
CREATE TYPE event_status AS ENUM (
    'new',
    'processing',
    'completed',
    'pending',
    'failed'
);

CREATE TYPE event_type AS ENUM(
    'create:team',
    'update:team',
    'delete:team',
    'create:jupyter',
    'update:jupyter',
    'delete:jupyter',
    'create:airflow',
    'update:airflow',
    'delete:airflow'
);

CREATE TYPE log_type as ENUM(
    'info',
    'warn',
    'error',
    'fatal'
);

CREATE TABLE Events (
    id uuid DEFAULT uuid_generate_v4(),
    event_type event_type NOT NULL,
    task JSONB NOT NULL,
    status event_status DEFAULT 'new' NOT NULL,
    deadline TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY(id)
);

CREATE TABLE Event_Logs (
    id uuid DEFAULT uuid_generate_v4(),
    event_id uuid NOT NULL,
    log_type log_type NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY(id),
    FOREIGN KEY (event_id) REFERENCES Events(id)
);

CREATE INDEX idx_event_logs_event_id ON Event_Logs(event_id);

-- +goose Down
DROP TABLE Event_Logs;

DROP TABLE Events;

DROP TYPE log_type;

DROP TYPE event_type;

DROP TYPE event_status;