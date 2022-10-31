// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.15.0
// source: charts_values.sql

package gensql

import (
	"context"
)

const chartValueInsert = `-- name: ChartValueInsert :exec
INSERT INTO chart_global_values (
    "key",
    "value",
    "chart_type"
) VALUES (
    $1,
    $2,
    $3
)
`

type ChartValueInsertParams struct {
	Key       string
	Value     string
	ChartType ChartType
}

func (q *Queries) ChartValueInsert(ctx context.Context, arg ChartValueInsertParams) error {
	_, err := q.db.ExecContext(ctx, chartValueInsert, arg.Key, arg.Value, arg.ChartType)
	return err
}

const valuesForChartGet = `-- name: ValuesForChartGet :many
SELECT id, created, key, value, chart_type
FROM chart_global_values
WHERE chart_type = $1
`

func (q *Queries) ValuesForChartGet(ctx context.Context, chartType ChartType) ([]ChartGlobalValue, error) {
	rows, err := q.db.QueryContext(ctx, valuesForChartGet, chartType)
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
