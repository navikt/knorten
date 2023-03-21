-- name: SessionCreate :exec
INSERT INTO "sessions" (
    "name",
    "email",
    "token",
    "access_token",
    "expires",
    "is_admin"
) VALUES (
    @name,
    @email,
    @token,
    @access_token,
    @expires,
    @is_admin
);

-- name: SessionGet :one
SELECT *
FROM "sessions"
WHERE token = @token
AND expires > now();

-- name: SessionDelete :exec
UPDATE "sessions"
SET "expires" = NOW()
WHERE token = @token;
