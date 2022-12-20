-- +goose Up
alter default privileges in schema public grant all on tables to cloudsqliamuser;
grant all on all tables in schema public to cloudsqliamuser;

-- +goose Down
alter default privileges in schema public revoke all on tables from cloudsqliamuser;
revoke all on all tables in schema public from cloudsqliamuser;