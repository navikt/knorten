package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/nais/knorten/pkg/database/gensql"
	"golang.org/x/exp/slices"
)

type AppService struct {
	App     string
	Ingress string
	Slug    string
}

type TeamServices struct {
	TeamID     string
	Slug       string
	Jupyterhub *AppService
	Airflow    *AppService
	Events     []Event
}

type ComputeService struct {
	Email  string
	Name   string
	Events []Event
}

type UserServices struct {
	Services []TeamServices
	Compute  *ComputeService
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

func createAppService(slug string, chartType gensql.ChartType) *AppService {
	return &AppService{
		App:     string(chartType),
		Ingress: createIngress(slug, chartType),
		Slug:    slug,
	}
}

func (r *Repo) AppsForTeamGet(ctx context.Context, teamID string) ([]gensql.ChartType, error) {
	return r.querier.AppsForTeamGet(ctx, teamID)
}

func (r *Repo) AppDelete(ctx context.Context, teamID string, chartType gensql.ChartType) error {
	return r.querier.AppDelete(ctx, gensql.AppDeleteParams{
		TeamID:    teamID,
		ChartType: chartType,
	})
}

func (r *Repo) ServicesForUser(ctx context.Context, email string) (UserServices, error) {
	teamsForUser, err := r.querier.TeamsForUserGet(ctx, email)
	if err != nil {
		return UserServices{}, err
	}

	slices.SortFunc(teamsForUser, func(a, b gensql.TeamsForUserGetRow) bool {
		return a.ID < b.ID
	})

	var userServices UserServices
	for _, team := range teamsForUser {
		apps, err := r.querier.AppsForTeamGet(ctx, team.ID)
		if err != nil {
			return UserServices{}, err
		}

		events, err := r.EventLogsForOwnerGet(ctx, team.ID)
		if err != nil {
			return UserServices{}, err
		}

		teamServices := TeamServices{
			TeamID: team.ID,
			Slug:   team.Slug,
			Events: events,
		}

		for _, app := range apps {
			switch app {
			case gensql.ChartTypeJupyterhub:
				teamServices.Jupyterhub = createAppService(team.Slug, app)
			case gensql.ChartTypeAirflow:
				teamServices.Airflow = createAppService(team.Slug, app)
			}
		}

		userServices.Services = append(userServices.Services, teamServices)
	}

	compute, err := r.querier.ComputeInstanceGet(ctx, email)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return UserServices{}, err
		}

		userServices.Compute = nil
	} else {
		events, err := r.EventLogsForOwnerGet(ctx, email)
		if err != nil {
			return UserServices{}, err
		}

		userServices.Compute = &ComputeService{
			Email:  compute.Email,
			Name:   compute.Name,
			Events: events,
		}
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
