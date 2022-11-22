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

-- name: AppsForTeamGet :many
SELECT DISTINCT ON (chart_type) chart_type
FROM chart_team_values
WHERE team_id = @team_id;

-- name: AppDelete :exec
DELETE FROM chart_team_values
WHERE team_id = @team_id AND chart_type = @chart_type;
