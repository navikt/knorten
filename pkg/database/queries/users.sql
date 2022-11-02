-- name: UserAppInsert :exec
INSERT INTO users ("email", "team", "chart_type")
VALUES (@email, @team, @chart_type);

-- name: UserAppsGet :many
SELECT team, chart_type FROM users where email = @email;