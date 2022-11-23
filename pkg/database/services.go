package database

import (
	"context"
	"fmt"

	"github.com/nais/knorten/pkg/database/gensql"
)

type Service struct {
	App            string
	Ingress        string
	Slug           string
	Secret         string
	ServiceAccount string
}

type TeamServices struct {
	TeamID     string
	Slug       string
	Jupyterhub *Service
	Airflow    *Service
}

func createIngress(team string, chartType gensql.ChartType) string {
	switch chartType {
	case gensql.ChartTypeJupyterhub:
		return fmt.Sprintf("https://%v.jupyter.knada.io", team)
	case gensql.ChartTypeAirflow:
		return fmt.Sprintf("https://%v.airflow.knada.io", team)
	}

	return ""
}

func createService(teamID, slug string, chartType gensql.ChartType) *Service {
	return &Service{
		App:            string(chartType),
		Ingress:        createIngress(slug, chartType),
		Slug:           slug,
		Secret:         fmt.Sprintf("https://console.cloud.google.com/security/secret-manager/secret/%v/versions?project=knada-gcp", teamID),
		ServiceAccount: fmt.Sprintf("%v@knada-gcp.iam.gserviceaccount.com", teamID),
	}
}

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

func (r *Repo) ServicesForUser(ctx context.Context, email string) ([]TeamServices, error) {
	teamsForUser, err := r.querier.TeamsForUserGet(ctx, email)
	if err != nil {
		return nil, err
	}

	var services []TeamServices
	for _, team := range teamsForUser {
		apps, err := r.querier.AppsForTeamGet(ctx, team.ID)
		if err != nil {
			return nil, err
		}

		teamServices := TeamServices{
			TeamID: team.ID,
			Slug:   team.Slug,
		}

		for _, app := range apps {
			switch app {
			case gensql.ChartTypeJupyterhub:
				teamServices.Jupyterhub = createService(team.ID, team.Slug, app)
			case gensql.ChartTypeAirflow:
				teamServices.Airflow = createService(team.ID, team.Slug, app)
			}
		}

		services = append(services, teamServices)
	}
	return services, nil
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
