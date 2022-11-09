-- name: UserAppInsert :exec
INSERT INTO users ("email", "team", "chart_type")
VALUES (@email, @team, @chart_type)
ON CONFLICT DO NOTHING;

-- name: UserAppsGet :many
SELECT team, chart_type
FROM users
where email = @email;

-- name: UserAppSetReady :exec
UPDATE users
SET ready = @ready
WHERE team = @team;
