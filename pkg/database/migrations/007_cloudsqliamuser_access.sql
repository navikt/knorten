-- +goose Up
-- +goose StatementBegin
DO
$$
    BEGIN
        IF EXISTS(SELECT * FROM pg_roles WHERE rolname = 'cloudsqliamuser') THEN
            alter default privileges in schema public grant all on tables to cloudsqliamuser;
            grant all on all tables in schema public to cloudsqliamuser;
        END IF;
    END
$$ LANGUAGE 'plpgsql';
-- +goose StatementEnd

-- +goose Down
DO
$$
    BEGIN
        IF EXISTS(SELECT * FROM pg_roles WHERE rolname = 'cloudsqliamuser') THEN
            alter default privileges in schema public revoke all on tables from cloudsqliamuser;
            revoke all on all tables in schema public from cloudsqliamuser;
        END IF;
    END
$$ LANGUAGE 'plpgsql';
