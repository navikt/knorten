-- +goose Up
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- +goose Down
DROP EXTENSION "uuid-ossp";
