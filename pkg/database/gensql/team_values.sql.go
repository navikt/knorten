// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.15.0
// source: team_values.sql

package gensql

import (
	"context"
)

const teamValueInsert = `-- name: TeamValueInsert :exec
INSERT INTO chart_team_values (
    "key",
    "value",
    "team",
    "chart_type"
) VALUES (
    $1,
    $2,
    $3,
    $4
)
`

type TeamValueInsertParams struct {
	Key       string
	Value     string
	Team      string
	ChartType ChartType
}

func (q *Queries) TeamValueInsert(ctx context.Context, arg TeamValueInsertParams) error {
	_, err := q.db.ExecContext(ctx, teamValueInsert,
		arg.Key,
		arg.Value,
		arg.Team,
		arg.ChartType,
	)
	return err
}

const teamValuesGet = `-- name: TeamValuesGet :many
SELECT DISTINCT ON ("key") id, created, key, value, chart_type, team
FROM chart_team_values
WHERE chart_type = $1 AND team = $2
ORDER BY "key", "created" DESC
`

type TeamValuesGetParams struct {
	ChartType ChartType
	Team      string
}

func (q *Queries) TeamValuesGet(ctx context.Context, arg TeamValuesGetParams) ([]ChartTeamValue, error) {
	rows, err := q.db.QueryContext(ctx, teamValuesGet, arg.ChartType, arg.Team)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ChartTeamValue{}
	for rows.Next() {
		var i ChartTeamValue
		if err := rows.Scan(
			&i.ID,
			&i.Created,
			&i.Key,
			&i.Value,
			&i.ChartType,
			&i.Team,
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
