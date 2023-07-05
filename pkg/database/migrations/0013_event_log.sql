-- +goose Up
CREATE TYPE event_status AS ENUM (
    'new',
    'processing',
    'completed',
    'failed',
    'invalid'
);

CREATE TYPE op AS ENUM('create', 'update', 'delete');

CREATE TYPE resource_type as ENUM('team', 'jupyter', 'airflow');

CREATE TABLE Events (
    id uuid DEFAULT uuid_generate_v4(),
    op op NOT NULL,
    resource_type resource_type NOT NULL,
    param JSONB NOT NULL,
    status event_status DEFAULT 'new' NOT NULL,
    deadline TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY(id)
);

CREATE TABLE Event_Logs (
    id uuid DEFAULT uuid_generate_v4(),
    event_id uuid NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY(id),
    FOREIGN KEY (event_id) REFERENCES Events(id)
);

CREATE INDEX idx_event_logs_event_id ON Event_Logs(event_id);

-- +goose Down
DROP TABLE Event_Logs;

DROP TABLE Events;

DROP TYPE resource_type;

DROP TYPE op;

DROP TYPE event_status;