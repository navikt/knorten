-- name: TeamValueInsert :exec
INSERT INTO chart_team_values (
    "key",
    "value",
    "team",
    "chart_type"
) VALUES (
    @key,
    @value,
    @team,
    @chart_type
);

-- name: TeamValuesGet :many
SELECT DISTINCT ON ("key") *
FROM chart_team_values
WHERE chart_type = @chart_type AND team = @team
ORDER BY "key", "created" DESC;
