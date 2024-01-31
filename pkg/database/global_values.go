package database

import (
	"context"

	"github.com/navikt/knorten/pkg/database/gensql"
)

func (r *Repo) GlobalChartValueInsert(ctx context.Context, key, value string, encrypted bool, chartType gensql.ChartType) error {
	return r.querier.GlobalValueInsert(ctx, gensql.GlobalValueInsertParams{
		Key:       key,
		Value:     value,
		ChartType: chartType,
		Encrypted: encrypted,
	})
}

func (r *Repo) GlobalValuesGet(ctx context.Context, chartType gensql.ChartType) ([]gensql.ChartGlobalValue, error) {
	return r.querier.GlobalValuesGet(ctx, chartType)
}

func (r *Repo) GlobalValueGet(ctx context.Context, chartType gensql.ChartType, key string) (gensql.ChartGlobalValue, error) {
	return r.querier.GlobalValueGet(ctx, gensql.GlobalValueGetParams{
		ChartType: chartType,
		Key:       key,
	})
}

func (r *Repo) GlobalValueDelete(ctx context.Context, key string, chartType gensql.ChartType) error {
	return r.querier.GlobalValueDelete(ctx, gensql.GlobalValueDeleteParams{
		Key:       key,
		ChartType: chartType,
	})
}
