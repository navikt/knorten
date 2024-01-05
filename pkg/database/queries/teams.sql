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
SELECT id, users, slug
FROM teams
WHERE id = @id;

-- name: TeamBySlugGet :one
SELECT id, users, slug
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
