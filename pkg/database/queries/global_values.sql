-- name: GlobalValueInsert :exec
INSERT INTO chart_global_values (
    "key",
    "value",
    "chart_type",
    "encrypted"
) VALUES (
    @key,
    @value,
    @chart_type,
    @encrypted
);

-- name: GlobalValuesGet :many
SELECT DISTINCT ON ("key") *
FROM chart_global_values
WHERE chart_type = @chart_type
ORDER BY "key", "created" DESC;

-- name: GlobalValueGet :one
SELECT DISTINCT ON ("key") *
FROM chart_global_values
WHERE chart_type = @chart_type AND "key" = @key
ORDER BY "created" DESC;

-- name: GlobalJupyterProfilesValueGet :one
SELECT DISTINCT ON ("key") *
FROM chart_global_values
WHERE "key" = 'singleuser.profileList'
ORDER BY "key", "created" DESC;

-- name: GlobalValueDelete :exec
DELETE
FROM chart_global_values
WHERE key = @key
  AND chart_type = @chart_type;
