// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.16.0
// source: teams.sql

package gensql

import (
	"context"

	"github.com/lib/pq"
)

const teamBySlugGet = `-- name: TeamBySlugGet :one
SELECT id, "owner", ("owner" || users)::text[] as users, slug, restrict_airflow_egress
FROM teams
WHERE slug = $1
`

type TeamBySlugGetRow struct {
	ID                    string
	Owner                 string
	Users                 []string
	Slug                  string
	RestrictAirflowEgress bool
}

func (q *Queries) TeamBySlugGet(ctx context.Context, slug string) (TeamBySlugGetRow, error) {
	row := q.db.QueryRowContext(ctx, teamBySlugGet, slug)
	var i TeamBySlugGetRow
	err := row.Scan(
		&i.ID,
		&i.Owner,
		pq.Array(&i.Users),
		&i.Slug,
		&i.RestrictAirflowEgress,
	)
	return i, err
}

const teamCreate = `-- name: TeamCreate :exec
INSERT INTO teams ("id", "users", "slug", "owner")
VALUES ($1, $2, $3, $4)
`

type TeamCreateParams struct {
	ID    string
	Users []string
	Slug  string
	Owner string
}

func (q *Queries) TeamCreate(ctx context.Context, arg TeamCreateParams) error {
	_, err := q.db.ExecContext(ctx, teamCreate,
		arg.ID,
		pq.Array(arg.Users),
		arg.Slug,
		arg.Owner,
	)
	return err
}

const teamDelete = `-- name: TeamDelete :exec
DELETE
FROM teams
WHERE id = $1
`

func (q *Queries) TeamDelete(ctx context.Context, id string) error {
	_, err := q.db.ExecContext(ctx, teamDelete, id)
	return err
}

const teamGet = `-- name: TeamGet :one
SELECT id, "owner", ("owner" || users)::text[] as users, slug, restrict_airflow_egress
FROM teams
WHERE id = $1
`

type TeamGetRow struct {
	ID                    string
	Owner                 string
	Users                 []string
	Slug                  string
	RestrictAirflowEgress bool
}

func (q *Queries) TeamGet(ctx context.Context, id string) (TeamGetRow, error) {
	row := q.db.QueryRowContext(ctx, teamGet, id)
	var i TeamGetRow
	err := row.Scan(
		&i.ID,
		&i.Owner,
		pq.Array(&i.Users),
		&i.Slug,
		&i.RestrictAirflowEgress,
	)
	return i, err
}

const teamSetAirflowRestrictEgress = `-- name: TeamSetAirflowRestrictEgress :exec
UPDATE teams
SET restrict_airflow_egress = $1
WHERE id = $2
`

type TeamSetAirflowRestrictEgressParams struct {
	RestrictAirflowEgress bool
	ID                    string
}

func (q *Queries) TeamSetAirflowRestrictEgress(ctx context.Context, arg TeamSetAirflowRestrictEgressParams) error {
	_, err := q.db.ExecContext(ctx, teamSetAirflowRestrictEgress, arg.RestrictAirflowEgress, arg.ID)
	return err
}

const teamUpdate = `-- name: TeamUpdate :exec
UPDATE teams
SET users      = $1
WHERE id = $2
`

type TeamUpdateParams struct {
	Users []string
	ID    string
}

func (q *Queries) TeamUpdate(ctx context.Context, arg TeamUpdateParams) error {
	_, err := q.db.ExecContext(ctx, teamUpdate, pq.Array(arg.Users), arg.ID)
	return err
}

const teamsForUserGet = `-- name: TeamsForUserGet :many
SELECT id, slug
FROM teams
WHERE "owner" = $1 OR $1::TEXT = ANY ("users")
`

type TeamsForUserGetRow struct {
	ID   string
	Slug string
}

func (q *Queries) TeamsForUserGet(ctx context.Context, email string) ([]TeamsForUserGetRow, error) {
	rows, err := q.db.QueryContext(ctx, teamsForUserGet, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []TeamsForUserGetRow{}
	for rows.Next() {
		var i TeamsForUserGetRow
		if err := rows.Scan(&i.ID, &i.Slug); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const teamsGet = `-- name: TeamsGet :many
select id, slug, users, created, restrict_airflow_egress, owner
from teams
ORDER BY slug
`

func (q *Queries) TeamsGet(ctx context.Context) ([]Team, error) {
	rows, err := q.db.QueryContext(ctx, teamsGet)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Team{}
	for rows.Next() {
		var i Team
		if err := rows.Scan(
			&i.ID,
			&i.Slug,
			pq.Array(&i.Users),
			&i.Created,
			&i.RestrictAirflowEgress,
			&i.Owner,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
