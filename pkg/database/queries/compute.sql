-- name: ComputeInstanceCreate :exec
INSERT INTO compute_instances ("owner", "name", "disk_size")
VALUES (@owner, @name, @disk_size);

-- name: ComputeInstanceUpdate :exec
UPDATE compute_instances
SET disk_size = @disk_size
WHERE "owner" = @owner;

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
