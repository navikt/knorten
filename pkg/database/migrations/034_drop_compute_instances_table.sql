-- +goose Up
DROP TABLE compute_instances;

-- +goose Down
CREATE TABLE compute_instances
(
    "owner"     TEXT NOT NULL,
    "name"      TEXT NOT NULL,
    "disk_size" INTEGER NOT NULL DEFAULT 10;
    PRIMARY KEY (team_id),
    CONSTRAINT fk_compute_instances_team
        FOREIGN KEY (team_id)
            REFERENCES teams (id) ON DELETE CASCADE
);
