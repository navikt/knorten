-- name: TeamCreate :exec
INSERT INTO teams ("id", "users", "slug", "api_access")
VALUES (@id, @users, @slug, @api_access);

-- name: TeamUpdate :exec
UPDATE teams
SET users      = @users,
    api_access = @api_access
WHERE id = @id;

-- name: TeamsForUserGet :many
SELECT id, slug
FROM teams
WHERE @email::TEXT = ANY ("users");

-- name: TeamGet :one
SELECT id, users, slug, pending_jupyter_upgrade, pending_airflow_upgrade, api_access, restrict_airflow_egress
FROM teams
WHERE slug = @slug;

-- name: TeamDelete :exec
DELETE
FROM teams
WHERE id = @id;

-- name: TeamsGet :many
select *
from teams
ORDER BY slug;

-- name: TeamSetPendingJupyterUpgrade :exec
UPDATE teams
SET pending_jupyter_upgrade = @pending_jupyter_upgrade
WHERE id = @id;

-- name: TeamSetPendingAirflowUpgrade :exec
UPDATE teams
SET pending_airflow_upgrade = @pending_airflow_upgrade
WHERE id = @id;

-- name: ClearPendingUpgradeLocks :exec
UPDATE teams
SET pending_jupyter_upgrade = false,
    pending_airflow_upgrade = false;

-- name: TeamSetAirflowRestrictEgress :exec
UPDATE teams
SET restrict_airflow_egress = @restrict_airflow_egress
WHERE id = @id;

-- name: TeamSetApiAccess :exec
UPDATE teams
SET api_access = @api_access
WHERE id = @id;