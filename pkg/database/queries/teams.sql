-- name: TeamCreate :exec
INSERT INTO teams ("id", "users", "slug", "enable_allowlist")
VALUES (@id, @users, @slug, @enable_allowlist);

-- name: TeamUpdate :exec
UPDATE teams
SET users = @users, enable_allowlist = @enable_allowlist
WHERE id = @id;

-- name: TeamsForUserGet :many
SELECT id, slug
FROM teams
WHERE @email::TEXT = ANY ("users");

-- name: TeamGet :one
SELECT id, users, slug, enable_allowlist
FROM teams
WHERE id = @id;

-- name: TeamBySlugGet :one
SELECT id, users, slug, enable_allowlist
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
