-- name: ComputeInstanceCreate :exec
INSERT INTO compute_instances ("email", "name")
VALUES (@email, @name);

-- name: ComputeInstanceGet :one
SELECT *
FROM compute_instances
WHERE email = @email;

-- name: ComputeInstanceDelete :exec
DELETE
FROM compute_instances
WHERE email = @email;
