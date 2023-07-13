-- name: ComputeInstanceCreate :exec
INSERT INTO compute_instances (
    "team_id",
    "instance_name",
    "machine_type"
) VALUES (
    @team_id,
    @instance_name,
    @machine_type
);

-- name: ComputeInstanceGet :one
SELECT *
FROM compute_instances
WHERE team_id = @team_id;

-- name: ComputeInstancesGet :many
SELECT *
FROM compute_instances;

-- name: SupportedComputeMachineTypes :many
SELECT unnest(enum_range(NULL::COMPUTE_MACHINE_TYPE))::text;

-- name: ComputeInstanceDelete :exec
DELETE
FROM compute_instances
WHERE team_id = @team_id;
