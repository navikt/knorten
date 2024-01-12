// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.25.0
// source: team_values.sql

package gensql

import (
	"context"
)

const chartDelete = `-- name: ChartDelete :exec
DELETE FROM chart_team_values
WHERE team_id = $1 AND chart_type = $2
`

type ChartDeleteParams struct {
	TeamID    string
	ChartType ChartType
}

func (q *Queries) ChartDelete(ctx context.Context, arg ChartDeleteParams) error {
	_, err := q.db.ExecContext(ctx, chartDelete, arg.TeamID, arg.ChartType)
	return err
}

const chartsForTeamGet = `-- name: ChartsForTeamGet :many
SELECT DISTINCT ON (chart_type) chart_type
FROM chart_team_values
WHERE team_id = $1
`

func (q *Queries) ChartsForTeamGet(ctx context.Context, teamID string) ([]ChartType, error) {
	rows, err := q.db.QueryContext(ctx, chartsForTeamGet, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ChartType{}
	for rows.Next() {
		var chart_type ChartType
		if err := rows.Scan(&chart_type); err != nil {
			return nil, err
		}
		items = append(items, chart_type)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const teamValueDelete = `-- name: TeamValueDelete :exec
DELETE FROM chart_team_values
WHERE key = $1 AND team_id = $2
`

type TeamValueDeleteParams struct {
	Key    string
	TeamID string
}

func (q *Queries) TeamValueDelete(ctx context.Context, arg TeamValueDeleteParams) error {
	_, err := q.db.ExecContext(ctx, teamValueDelete, arg.Key, arg.TeamID)
	return err
}

const teamValueGet = `-- name: TeamValueGet :one
SELECT DISTINCT ON ("key") id, created, key, value, chart_type, team_id
FROM chart_team_values
WHERE key = $1
  AND team_id = $2
ORDER BY "key", "created" DESC
`

type TeamValueGetParams struct {
	Key    string
	TeamID string
}

func (q *Queries) TeamValueGet(ctx context.Context, arg TeamValueGetParams) (ChartTeamValue, error) {
	row := q.db.QueryRowContext(ctx, teamValueGet, arg.Key, arg.TeamID)
	var i ChartTeamValue
	err := row.Scan(
		&i.ID,
		&i.Created,
		&i.Key,
		&i.Value,
		&i.ChartType,
		&i.TeamID,
	)
	return i, err
}

const teamValueInsert = `-- name: TeamValueInsert :exec
INSERT INTO chart_team_values ("key",
                               "value",
                               "team_id",
                               "chart_type")
VALUES ($1,
        $2,
        $3,
        $4)
`

type TeamValueInsertParams struct {
	Key       string
	Value     string
	TeamID    string
	ChartType ChartType
}

func (q *Queries) TeamValueInsert(ctx context.Context, arg TeamValueInsertParams) error {
	_, err := q.db.ExecContext(ctx, teamValueInsert,
		arg.Key,
		arg.Value,
		arg.TeamID,
		arg.ChartType,
	)
	return err
}

const teamValuesGet = `-- name: TeamValuesGet :many
SELECT DISTINCT ON ("key") id, created, key, value, chart_type, team_id
FROM chart_team_values
WHERE chart_type = $1
  AND team_id = $2
ORDER BY "key", "created" DESC
`

type TeamValuesGetParams struct {
	ChartType ChartType
	TeamID    string
}

func (q *Queries) TeamValuesGet(ctx context.Context, arg TeamValuesGetParams) ([]ChartTeamValue, error) {
	rows, err := q.db.QueryContext(ctx, teamValuesGet, arg.ChartType, arg.TeamID)
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
			&i.TeamID,
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

const teamsForChartGet = `-- name: TeamsForChartGet :many
SELECT DISTINCT ON (team_id) team_id
FROM chart_team_values
WHERE chart_type = $1
`

func (q *Queries) TeamsForChartGet(ctx context.Context, chartType ChartType) ([]string, error) {
	rows, err := q.db.QueryContext(ctx, teamsForChartGet, chartType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var team_id string
		if err := rows.Scan(&team_id); err != nil {
			return nil, err
		}
		items = append(items, team_id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
