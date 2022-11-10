package database

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
)

func (r *Repo) TeamCreate(ctx context.Context, chartValues map[string]string, team string, users []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	querier := r.querier.WithTx(tx)
	err = r.querier.TeamCreate(ctx, gensql.TeamCreateParams{
		Team:  team,
		Users: users,
	})
	if err != nil {
		if err := tx.Rollback(); err != nil {
			r.log.WithError(err).Error("rolling back namespace create transaction - team create")
		}
		return err
	}

	for key, value := range chartValues {
		err := querier.TeamValueInsert(ctx, gensql.TeamValueInsertParams{
			Key:       key,
			Value:     value,
			Team:      team,
			ChartType: gensql.ChartTypeNamespace,
		})
		if err != nil {
			if err := tx.Rollback(); err != nil {
				r.log.WithError(err).Error("rolling back namespace create transaction - team chart value insert")
			}
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *Repo) TeamUpdate(ctx context.Context, team string, users []string) error {
	return r.querier.TeamUpdate(ctx, gensql.TeamUpdateParams{
		Team:  team,
		Users: users,
	})
}

func (r *Repo) TeamGet(ctx context.Context, team string) (gensql.TeamGetRow, error) {
	return r.querier.TeamGet(ctx, team)
}

func (r *Repo) TeamDelete(ctx context.Context, team string) error {
	return r.querier.TeamDelete(ctx, team)
}
