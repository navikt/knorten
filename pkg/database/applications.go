package database

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
)

func (r *Repo) ApplicationCreate(ctx context.Context, chartType gensql.ChartType, chartValues []*helm.ChartValue, namespace string, users []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	querier := r.querier.WithTx(tx)
	for _, value := range chartValues {
		err := querier.TeamValueInsert(ctx, gensql.TeamValueInsertParams{
			Key:       value.Key,
			Value:     value.Value,
			Team:      namespace,
			ChartType: chartType,
		})
		if err != nil {
			if err := tx.Rollback(); err != nil {
				r.log.WithError(err).Error("rolling back application create transaction - team chart value insert")
			}
			return err
		}
	}

	for _, user := range users {
		err := querier.UserAppInsert(ctx, gensql.UserAppInsertParams{
			Email:     user,
			Team:      namespace,
			ChartType: chartType,
		})
		if err != nil {
			if err := tx.Rollback(); err != nil {
				r.log.WithError(err).Error("rolling back application create transaction - user app insert")
			}
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
