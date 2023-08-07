-- name: TeamValueInsert :exec
INSERT INTO chart_team_values ("key",
                               "value",
                               "team_id",
                               "chart_type")
VALUES (@key,
        @value,
        @team_id,
        @chart_type);

-- name: TeamValuesGet :many
SELECT DISTINCT ON ("key") *
FROM chart_team_values
WHERE chart_type = @chart_type
  AND team_id = @team_id
ORDER BY "key", "created" DESC;

-- name: TeamValueGet :one
SELECT DISTINCT ON ("key") *
FROM chart_team_values
WHERE key = @key
  AND team_id = @team_id
ORDER BY "key", "created" DESC;

-- name: TeamValueDelete :exec
DELETE FROM chart_team_values
WHERE key = @key AND team_id = @team_id;

-- name: ChartsForTeamGet :many
SELECT DISTINCT ON (chart_type) chart_type
FROM chart_team_values
WHERE team_id = @team_id;

-- name: TeamsForChartGet :many
SELECT DISTINCT ON (team_id) team_id
FROM chart_team_values
WHERE chart_type = @chart_type;

-- name: ChartDelete :exec
DELETE FROM chart_team_values
WHERE team_id = @team_id AND chart_type = @chart_type;
