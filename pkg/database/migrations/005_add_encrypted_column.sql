-- +goose Up
ALTER TABLE chart_global_values ADD COLUMN "encrypted" BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE chart_global_values DROP column "encrypted";
