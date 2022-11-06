package database

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
)

func (r *Repo) UserAppInsert(ctx context.Context, email, team string, chartType gensql.ChartType) error {
	return r.querier.UserAppInsert(ctx, gensql.UserAppInsertParams{
		Email:     email,
		Team:      team,
		ChartType: chartType,
	})
}

func (r *Repo) UserAppsGet(ctx context.Context, email string) ([]gensql.UserAppsGetRow, error) {
	return r.querier.UserAppsGet(ctx, email)
}

func (r *Repo) UserAppSetReady(ctx context.Context, team string, ready bool) error {
	return r.querier.UserAppSetReady(ctx, gensql.UserAppSetReadyParams{
		Team:  team,
		Ready: ready,
	})
}
