// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.16.0
// source: global_values.sql

package gensql

import (
	"context"
)

const globalJupyterProfilesValueGet = `-- name: GlobalJupyterProfilesValueGet :one
SELECT DISTINCT ON ("key") id, created, key, value, chart_type, encrypted
FROM chart_global_values
WHERE "key" = 'singleuser.profileList'
ORDER BY "key", "created" DESC
`

func (q *Queries) GlobalJupyterProfilesValueGet(ctx context.Context) (ChartGlobalValue, error) {
	row := q.db.QueryRowContext(ctx, globalJupyterProfilesValueGet)
	var i ChartGlobalValue
	err := row.Scan(
		&i.ID,
		&i.Created,
		&i.Key,
		&i.Value,
		&i.ChartType,
		&i.Encrypted,
	)
	return i, err
}

const globalValueDelete = `-- name: GlobalValueDelete :exec
DELETE
FROM chart_global_values
WHERE key = $1
  AND chart_type = $2
`

type GlobalValueDeleteParams struct {
	Key       string
	ChartType ChartType
}

func (q *Queries) GlobalValueDelete(ctx context.Context, arg GlobalValueDeleteParams) error {
	_, err := q.db.ExecContext(ctx, globalValueDelete, arg.Key, arg.ChartType)
	return err
}

const globalValueGet = `-- name: GlobalValueGet :one
SELECT DISTINCT ON ("key") id, created, key, value, chart_type, encrypted
FROM chart_global_values
WHERE chart_type = $1 AND "key" = $2
ORDER BY "key", "created" DESC
`

type GlobalValueGetParams struct {
	ChartType ChartType
	Key       string
}

func (q *Queries) GlobalValueGet(ctx context.Context, arg GlobalValueGetParams) (ChartGlobalValue, error) {
	row := q.db.QueryRowContext(ctx, globalValueGet, arg.ChartType, arg.Key)
	var i ChartGlobalValue
	err := row.Scan(
		&i.ID,
		&i.Created,
		&i.Key,
		&i.Value,
		&i.ChartType,
		&i.Encrypted,
	)
	return i, err
}

const globalValueInsert = `-- name: GlobalValueInsert :exec
INSERT INTO chart_global_values (
    "key",
    "value",
    "chart_type",
    "encrypted"
) VALUES (
    $1,
    $2,
    $3,
    $4
)
`

type GlobalValueInsertParams struct {
	Key       string
	Value     string
	ChartType ChartType
	Encrypted bool
}

func (q *Queries) GlobalValueInsert(ctx context.Context, arg GlobalValueInsertParams) error {
	_, err := q.db.ExecContext(ctx, globalValueInsert,
		arg.Key,
		arg.Value,
		arg.ChartType,
		arg.Encrypted,
	)
	return err
}

const globalValuesGet = `-- name: GlobalValuesGet :many
SELECT DISTINCT ON ("key") id, created, key, value, chart_type, encrypted
FROM chart_global_values
WHERE chart_type = $1
ORDER BY "key", "created" DESC
`

func (q *Queries) GlobalValuesGet(ctx context.Context, chartType ChartType) ([]ChartGlobalValue, error) {
	rows, err := q.db.QueryContext(ctx, globalValuesGet, chartType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ChartGlobalValue{}
	for rows.Next() {
		var i ChartGlobalValue
		if err := rows.Scan(
			&i.ID,
			&i.Created,
			&i.Key,
			&i.Value,
			&i.ChartType,
			&i.Encrypted,
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
