-- +goose Up
-- +goose StatementBegin
DO
$$
    BEGIN
        IF EXISTS(SELECT * FROM pg_roles WHERE rolname = 'knada_metrics') THEN
            ALTER DEFAULT privileges IN SCHEMA public GRANT select ON tables TO knada_metrics;
            GRANT select ON all tables IN SCHEMA public TO knada_metrics;
        END IF;
    END
$$ LANGUAGE 'plpgsql';
-- +goose StatementEnd

ALTER TABLE teams ADD COLUMN "uid" uuid NOT NULL DEFAULT uuid_generate_v4();

-- +goose Down
-- +goose StatementBegin
DO
$$
    BEGIN
        IF EXISTS(SELECT * FROM pg_roles WHERE rolname = 'knada_metrics') THEN
            ALTER DEFAULT privileges IN SCHEMA public REVOKE select ON tables FROM knada_metrics;
            REVOKE select ON all tables IN SCHEMA public FROM knada_metrics;
        END IF;
    END
$$ LANGUAGE 'plpgsql';
-- +goose StatementEnd

ALTER TABLE teams DROP COLUMN "uid";
