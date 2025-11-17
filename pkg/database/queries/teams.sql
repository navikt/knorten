-- name: TeamCreate :exec
INSERT INTO teams ("id", "users", "slug", "teamkatalogen_team")
VALUES (@id, @users, @slug, @teamkatalogen_team);

-- name: TeamUpdate :exec
UPDATE teams
SET users = @users,
    teamkatalogen_team = @teamkatalogen_team
WHERE id = @id;

-- name: TeamsForUserGet :many
SELECT id, slug, teamkatalogen_team
FROM teams
WHERE @email::TEXT = ANY ("users");

-- name: TeamGet :one
SELECT id, users, slug, teamkatalogen_team
FROM teams
WHERE id = @id;

-- name: TeamBySlugGet :one
SELECT id, users, slug, teamkatalogen_team
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
