-- name: SessionCreate :exec
INSERT INTO "sessions" (
    "name",
    "email",
    "token",
    "access_token",
    "expires"
) VALUES (
    @name,
    @email,
    @token,
    @access_token,
    @expires
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
