-- +goose Up
ALTER TABLE events
    ALTER event_type TYPE TEXT,
    ALTER status TYPE TEXT,
    ALTER "status" SET DEFAULT 'new';
ALTER TABLE Event_Logs ALTER COLUMN log_type TYPE TEXT;

ALTER TABLE events RENAME COLUMN event_type TO type;

-- +goose Down
ALTER TABLE events
    ALTER type TYPE event_type USING type::event_type,
    ALTER status TYPE event_status USING status::event_status,
    ALTER "status" SET DEFAULT 'new';
ALTER TABLE Event_Logs ALTER COLUMN log_type TYPE log_type USING log_type::log_type;

ALTER TABLE events RENAME COLUMN type TO event_type;
