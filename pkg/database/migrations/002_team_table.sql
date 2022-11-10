-- +goose Up
CREATE TABLE teams
(
    "team"            TEXT         NOT NULL,
    "users"           TEXT[]       NOT NULL,
    "created"         TIMESTAMPTZ  DEFAULT NOW(),
    PRIMARY KEY (team)
);

-- +goose Down
DROP TABLE teams;
