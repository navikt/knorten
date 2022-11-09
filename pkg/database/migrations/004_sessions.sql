-- +goose Up
CREATE TABLE sessions (
    "token"        TEXT        NOT NULL,
    "access_token" TEXT        NOT NULL,
    "email"        TEXT        NOT NULL,
    "name"         TEXT        NOT NULL,
    "created"      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "expires"      TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (token)
);

-- +goose Down
DROP TABLE sessions;
