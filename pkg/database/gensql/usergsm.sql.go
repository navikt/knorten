// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.16.0
// source: usergsm.sql

package gensql

import (
	"context"
)

const userGoogleSecretManagerCreate = `-- name: UserGoogleSecretManagerCreate :exec
INSERT INTO user_google_secret_manager ("owner", "name")
VALUES ($1, $2)
`

type UserGoogleSecretManagerCreateParams struct {
	Owner string
	Name  string
}

func (q *Queries) UserGoogleSecretManagerCreate(ctx context.Context, arg UserGoogleSecretManagerCreateParams) error {
	_, err := q.db.ExecContext(ctx, userGoogleSecretManagerCreate, arg.Owner, arg.Name)
	return err
}

const userGoogleSecretManagerDelete = `-- name: UserGoogleSecretManagerDelete :exec
DELETE
FROM user_google_secret_manager
WHERE owner = $1
`

func (q *Queries) UserGoogleSecretManagerDelete(ctx context.Context, owner string) error {
	_, err := q.db.ExecContext(ctx, userGoogleSecretManagerDelete, owner)
	return err
}

const userGoogleSecretManagerGet = `-- name: UserGoogleSecretManagerGet :one
SELECT owner, name
FROM user_google_secret_manager
WHERE owner = $1
`

func (q *Queries) UserGoogleSecretManagerGet(ctx context.Context, owner string) (UserGoogleSecretManager, error) {
	row := q.db.QueryRowContext(ctx, userGoogleSecretManagerGet, owner)
	var i UserGoogleSecretManager
	err := row.Scan(&i.Owner, &i.Name)
	return i, err
}
