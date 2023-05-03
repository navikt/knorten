package database

import (
	"context"
	"strings"

	"github.com/nais/knorten/pkg/database/gensql"
)

func (r *Repo) TeamCreate(ctx context.Context, team, slug, owner string, users []string, apiAccess bool) error {
	return r.querier.TeamCreate(ctx, gensql.TeamCreateParams{
		ID:        team,
		Users:     stringSliceToLower(users),
		Slug:      slug,
		ApiAccess: apiAccess,
		Owner:     owner,
	})
}

func (r *Repo) TeamUpdate(ctx context.Context, team string, users []string, apiAccess bool) error {
	return r.querier.TeamUpdate(ctx, gensql.TeamUpdateParams{
		ID:        team,
		Users:     stringSliceToLower(users),
		ApiAccess: apiAccess,
	})
}

func (r *Repo) TeamGet(ctx context.Context, slug string) (gensql.TeamGetRow, error) {
	return r.querier.TeamGet(ctx, slug)
}

func (r *Repo) TeamDelete(ctx context.Context, team string) error {
	return r.querier.TeamDelete(ctx, team)
}

func (r *Repo) TeamsGet(ctx context.Context) ([]gensql.Team, error) {
	return r.querier.TeamsGet(ctx)
}

func (r *Repo) TeamsForAppGet(ctx context.Context, chartType gensql.ChartType) ([]string, error) {
	return r.querier.TeamsForAppGet(ctx, chartType)
}

func (r *Repo) TeamSetPendingUpgrade(ctx context.Context, teamID, chartType string, pendingUpgrade bool) error {
	var err error
	switch chartType {
	case string(gensql.ChartTypeJupyterhub):
		err = r.querier.TeamSetPendingJupyterUpgrade(ctx, gensql.TeamSetPendingJupyterUpgradeParams{
			ID:                    teamID,
			PendingJupyterUpgrade: pendingUpgrade,
		})
	case string(gensql.ChartTypeAirflow):
		err = r.querier.TeamSetPendingAirflowUpgrade(ctx, gensql.TeamSetPendingAirflowUpgradeParams{
			ID:                    teamID,
			PendingAirflowUpgrade: pendingUpgrade,
		})
	}
	return err
}

func (r *Repo) TeamSetRestrictAirflowEgress(ctx context.Context, teamID string, restrictAirflowEgress bool) error {
	return r.querier.TeamSetAirflowRestrictEgress(ctx, gensql.TeamSetAirflowRestrictEgressParams{
		RestrictAirflowEgress: restrictAirflowEgress,
		ID:                    teamID,
	})
}

func (r *Repo) TeamSetApiAccess(ctx context.Context, teamID string, apiAccess bool) error {
	return r.querier.TeamSetApiAccess(ctx, gensql.TeamSetApiAccessParams{
		ApiAccess: apiAccess,
		ID:        teamID,
	})
}

func stringSliceToLower(vals []string) []string {
	out := []string{}
	for _, v := range vals {
		out = append(out, strings.ToLower(v))
	}

	return out
}
