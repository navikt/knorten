// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.15.0
// source: sessions.sql

package gensql

import (
	"context"
	"time"
)

const sessionCreate = `-- name: SessionCreate :exec
INSERT INTO "sessions" (
    "name",
    "email",
    "token",
    "access_token",
    "expires"
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
`

type SessionCreateParams struct {
	Name        string
	Email       string
	Token       string
	AccessToken string
	Expires     time.Time
}

func (q *Queries) SessionCreate(ctx context.Context, arg SessionCreateParams) error {
	_, err := q.db.ExecContext(ctx, sessionCreate,
		arg.Name,
		arg.Email,
		arg.Token,
		arg.AccessToken,
		arg.Expires,
	)
	return err
}

const sessionDelete = `-- name: SessionDelete :exec
UPDATE "sessions"
SET "expires" = NOW()
WHERE token = $1
`

func (q *Queries) SessionDelete(ctx context.Context, token string) error {
	_, err := q.db.ExecContext(ctx, sessionDelete, token)
	return err
}

const sessionGet = `-- name: SessionGet :one
SELECT token, access_token, email, name, created, expires
FROM "sessions"
WHERE token = $1
AND expires > now()
`

func (q *Queries) SessionGet(ctx context.Context, token string) (Session, error) {
	row := q.db.QueryRowContext(ctx, sessionGet, token)
	var i Session
	err := row.Scan(
		&i.Token,
		&i.AccessToken,
		&i.Email,
		&i.Name,
		&i.Created,
		&i.Expires,
	)
	return i, err
}
