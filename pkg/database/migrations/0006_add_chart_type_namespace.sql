-- +goose Up
ALTER TYPE CHART_TYPE ADD VALUE 'namespace';

-- +goose Down
ALTER TYPE CHART_TYPE REMOVE VALUE 'namespace';