-- +goose Up
DROP TABLE compute_instances;
DROP TYPE COMPUTE_MACHINE_TYPE;

CREATE TABLE compute_instances
(
    "email" TEXT PRIMARY KEY,
    "name"  TEXT NOT NULL
);

-- +goose Down
DROP TABLE compute_instances;

CREATE TYPE COMPUTE_MACHINE_TYPE AS ENUM ('e2-standard-4', 'n2-standard-2', 'c2-standard-4');

CREATE TABLE compute_instances
(
    "team_id"       TEXT                 NOT NULL,
    "instance_name" TEXT                 NOT NULL,
    "machine_type"  COMPUTE_MACHINE_TYPE NOT NULL,
    PRIMARY KEY (team_id),
    CONSTRAINT fk_compute_instances_team
        FOREIGN KEY (team_id)
            REFERENCES teams (id) ON DELETE CASCADE
);
