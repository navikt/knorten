-- +goose Up
CREATE TABLE teams
(
    "id"      TEXT   NOT NULL,
    "slug"    TEXT   NOT NULL,
    "users"   TEXT[] NOT NULL,
    "created" TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (id)
);

-- +goose Down
DROP TABLE teams;
