-- name: TeamCreate :exec
INSERT INTO teams ("id", "users", "slug")
VALUES (@id, @users, @slug);

-- name: TeamUpdate :exec
UPDATE teams
SET users = @users
WHERE id = @id;

-- name: TeamsForUserGet :many
SELECT id, slug
FROM teams
WHERE @email::TEXT = ANY ("users");

-- name: TeamGet :one
SELECT id, users, slug, pending_jupyter_upgrade, pending_airflow_upgrade
FROM teams
WHERE slug = @slug;

-- name: TeamDelete :exec
DELETE
FROM teams
WHERE id = @id;

-- name: TeamsGet :many
select id, users, slug
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
SET pending_jupyter_upgrade = false, pending_airflow_upgrade = false;
