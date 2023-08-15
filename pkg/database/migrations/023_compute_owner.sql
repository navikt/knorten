-- +goose Up
ALTER TABLE compute_instances RENAME COLUMN email TO owner;

-- +goose Down
ALTER TABLE compute_instances RENAME COLUMN owner TO email;
