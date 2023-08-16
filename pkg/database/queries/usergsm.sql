-- name: UserGoogleSecretManagerCreate :exec
INSERT INTO user_google_secret_manager ("owner", "name")
VALUES (@owner, @name);

-- name: UserGoogleSecretManagerGet :one
SELECT *
FROM user_google_secret_manager
WHERE owner = @owner;

-- name: UserGoogleSecretManagerDelete :exec
DELETE
FROM user_google_secret_manager
WHERE owner = @owner;
