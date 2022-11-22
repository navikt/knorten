package database

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
)

func (r *Repo) AppsForTeamGet(ctx context.Context, team string) ([]string, error) {
	get, err := r.querier.AppsForTeamGet(ctx, team)
	if err != nil {
		return nil, err
	}

	apps := make([]string, len(get))
	for i, chartType := range get {
		apps[i] = string(chartType)
	}

	return apps, nil
}

func (r *Repo) AppDelete(ctx context.Context, teamID string, chartType gensql.ChartType) error {
	return r.querier.AppDelete(ctx, gensql.AppDeleteParams{
		TeamID:    teamID,
		ChartType: chartType,
	})
}

func (r *Repo) ServicesForUser(ctx context.Context, email string) (map[string][]gensql.ChartType, error) {
	teamsSQL, err := r.querier.TeamsForUserGet(ctx, email)
	if err != nil {
		return nil, err
	}

	userServices := map[string][]gensql.ChartType{}
	for _, team := range teamsSQL {
		userServices[team.Slug] = []gensql.ChartType{}
		servicesForTeam, err := r.querier.AppsForTeamGet(ctx, team.ID)
		if err != nil {
			return nil, err
		}
		userServices[team.Slug] = append(userServices[team.Slug], servicesForTeam...)
	}
	return userServices, nil
}

func (r *Repo) TeamValuesInsert(ctx context.Context, chartType gensql.ChartType, chartValues map[string]string, team string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	querier := r.querier.WithTx(tx)
	for key, value := range chartValues {
		err := querier.TeamValueInsert(ctx, gensql.TeamValueInsertParams{
			Key:       key,
			Value:     value,
			TeamID:    team,
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
