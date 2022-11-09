-- +goose Up
CREATE TABLE users
(
    "id"              uuid        DEFAULT uuid_generate_v4(),
    "created"         TIMESTAMPTZ DEFAULT NOW(),
    "email"           TEXT       NOT NULL,
    "team"            TEXT       NOT NULL,
    "chart_type"      CHART_TYPE NOT NULL,
    PRIMARY KEY (id)
);

-- +goose Down
DROP TABLE users;