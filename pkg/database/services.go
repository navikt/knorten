package database

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
)

func (r *Repo) ServicesForUser(ctx context.Context, email string) (map[string][]gensql.ChartType, error) {
	teamsSQL, err := r.querier.TeamsForUserGet(ctx, email)
	if err != nil {
		return nil, err
	}

	userServices := map[string][]gensql.ChartType{}
	for _, t := range teamsSQL {
		servicesForTeam, err := r.querier.AppsForTeamGet(ctx, t)
		if err != nil {
			return nil, err
		}
		for _, s := range servicesForTeam {
			userServices[t] = append(userServices[t], s.ChartType)
		}
	}
	return userServices, nil
}

func (r *Repo) ServiceCreate(ctx context.Context, chartType gensql.ChartType, chartValues map[string]string, namespace string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	querier := r.querier.WithTx(tx)
	for key, value := range chartValues {
		err := querier.TeamValueInsert(ctx, gensql.TeamValueInsertParams{
			Key:       key,
			Value:     value,
			Team:      namespace,
			ChartType: chartType,
		})
		if err != nil {
			if err := tx.Rollback(); err != nil {
				r.log.WithError(err).Error("rolling back service create transaction - team chart value insert")
			}
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
