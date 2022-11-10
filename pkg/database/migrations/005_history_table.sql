-- +goose Up
CREATE TABLE chart_values_history
(
    id         serial,
    tstamp     TIMESTAMPTZ DEFAULT now(),
    table_name TEXT,
    operation  TEXT,
    team       TEXT,
    chart_type CHART_TYPE,
    key        TEXT,
    new_value  TEXT,
    old_value  TEXT,

    PRIMARY KEY (id)
);

-- +goose StatementBegin
CREATE FUNCTION change_trigger() RETURNS trigger AS
$$
BEGIN
    IF TG_OP = 'INSERT'
    THEN
        INSERT INTO chart_values_history (table_name, operation, team, chart_type, key, new_value)
        VALUES (TG_TABLE_NAME, TG_OP, NEW.team, NEW.chart_type, NEW.key, NEW.value);
        RETURN NEW;
    ELSIF TG_OP = 'UPDATE'
    THEN
        INSERT INTO chart_values_history (table_name, operation, team, chart_type, key, new_value, old_value)
        VALUES (TG_TABLE_NAME, TG_OP, NEW.team, NEW.chart_type, NEW.key, NEW.value, OLD.value);
        RETURN NEW;
    ELSIF TG_OP = 'DELETE'
    THEN
        INSERT INTO chart_values_history (table_name, operation, team, chart_type, key, old_value)
        VALUES (TG_TABLE_NAME, TG_OP, OLD.team, OLD.chart_type, OLD.key, OLD.value);
        RETURN OLD;
    END IF;
END;
$$ LANGUAGE 'plpgsql';
-- +goose StatementEnd

CREATE TRIGGER values_history
    BEFORE INSERT OR UPDATE OR DELETE
    ON chart_team_values
    FOR EACH ROW
EXECUTE PROCEDURE change_trigger();

ALTER TABLE chart_team_values
    ADD CONSTRAINT new_value UNIQUE (key, value, chart_type, team);

-- +goose Down
ALTER TABLE chart_team_values
    DROP CONSTRAINT new_value;

DROP TRIGGER values_history on chart_team_values;

DROP FUNCTION change_trigger;

DROP TABLE chart_values_history;
