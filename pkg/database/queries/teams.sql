-- name: TeamCreate :exec
INSERT INTO teams ("team", "users")
VALUES (@team, @users);

-- name: TeamUpdate :exec
UPDATE teams
SET users = @users
WHERE team = @team;

-- name: TeamsForUserGet :many
SELECT team FROM teams
WHERE @email::TEXT = ANY("users");

-- name: TeamGet :one
SELECT team, users FROM teams
WHERE team = @team;

-- name: TeamDelete :exec
DELETE FROM teams
WHERE team = @team;
