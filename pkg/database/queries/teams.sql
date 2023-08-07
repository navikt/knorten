-- name: TeamCreate :exec
INSERT INTO teams ("id", "users", "slug", "owner")
VALUES (@id, @users, @slug, @owner);

-- name: TeamUpdate :exec
UPDATE teams
SET users      = @users
WHERE id = @id;

-- name: TeamsForUserGet :many
SELECT id, slug
FROM teams
WHERE "owner" = @email OR @email::TEXT = ANY ("users");

-- name: TeamGet :one
SELECT id, "owner", ("owner" || users)::text[] as users, slug, pending_jupyter_upgrade, pending_airflow_upgrade, restrict_airflow_egress
FROM teams
WHERE id = @id;

-- name: TeamBySlugGet :one
SELECT id, "owner", ("owner" || users)::text[] as users, slug, restrict_airflow_egress
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
