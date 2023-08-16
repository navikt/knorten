-- +goose Up

CREATE TABLE user_google_secret_manager
(
    "owner" TEXT PRIMARY KEY,
    "name"  TEXT NOT NULL
);

-- +goose Down
DROP TABLE user_google_secret_manager;
