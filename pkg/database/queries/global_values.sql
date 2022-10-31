-- name: GlobalValueInsert :exec
INSERT INTO chart_global_values (
    "key",
    "value",
    "chart_type"
) VALUES (
    @key,
    @value,
    @chart_type
);

-- name: GlobalValuesGet :many
SELECT DISTINCT ON ("key") *
FROM chart_global_values
WHERE chart_type = @chart_type
ORDER BY "key", "created" DESC;
