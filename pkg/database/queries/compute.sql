-- name: ComputeInstanceCreate :exec
INSERT INTO compute_instances ("owner", "name")
VALUES (@owner, @name);

-- name: ComputeInstancesGet :many
SELECT *
FROM compute_instances;

-- name: ComputeInstanceGet :one
SELECT *
FROM compute_instances
WHERE owner = @owner;

-- name: ComputeInstanceDelete :exec
DELETE
FROM compute_instances
WHERE owner = @owner;
