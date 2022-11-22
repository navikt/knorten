package database

import (
	"context"
	"github.com/nais/knorten/pkg/database/gensql"
)

func (r *Repo) TeamCreate(ctx context.Context, team, slug string, users []string) error {
	return r.querier.TeamCreate(ctx, gensql.TeamCreateParams{
		ID:    team,
		Users: users,
		Slug:  slug,
	})
}

func (r *Repo) TeamUpdate(ctx context.Context, team string, users []string) error {
	return r.querier.TeamUpdate(ctx, gensql.TeamUpdateParams{
		ID:    team,
		Users: users,
	})
}

func (r *Repo) TeamGet(ctx context.Context, team string) (gensql.TeamGetRow, error) {
	return r.querier.TeamGet(ctx, team)
}

func (r *Repo) TeamDelete(ctx context.Context, team string) error {
	return r.querier.TeamDelete(ctx, team)
}
