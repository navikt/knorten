package team

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/navikt/knorten/pkg/k8s/core"

	"github.com/navikt/knorten/pkg/chart"
	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/k8s"
)

var (
	ErrTeamExists = errors.New("team with slug already exists")
)

type Client struct {
	repo       *database.Repo
	manager    k8s.Manager
	gcpProject string
	gcpRegion  string
	dryRun     bool
}

func NewClient(repo *database.Repo, mngr k8s.Manager, gcpProject, gcpRegion string, dryRun bool) (*Client, error) {
	return &Client{
		repo:       repo,
		manager:    mngr,
		gcpProject: gcpProject,
		gcpRegion:  gcpRegion,
		dryRun:     dryRun,
	}, nil
}

func (c Client) Create(ctx context.Context, team *gensql.Team) error {
	existingTeam, err := c.repo.TeamBySlugGet(ctx, team.Slug)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("retrieving team by slug: %w", err)
	}

	if existingTeam.Slug == team.Slug {
		return ErrTeamExists
	}

	if err := c.createGCPTeamResources(ctx, team); err != nil {
		return fmt.Errorf("creating GCP resources: %w", err)
	}

	namespace := k8s.TeamIDToNamespace(team.ID)
	err = c.manager.ApplyNamespace(ctx, core.NewNamespace(namespace, core.WithTeamNamespaceLabel()))
	if err != nil {
		return fmt.Errorf("creating k8s namespace: %w", err)
	}

	if err := c.manager.ApplyServiceAccount(ctx, core.NewServiceAccount(team.ID, namespace)); err != nil {
		return fmt.Errorf("creating k8s service account: %w", err)
	}

	if err := c.repo.TeamCreate(ctx, team); err != nil {
		return fmt.Errorf("saving team to database: %w", err)
	}

	return nil
}

func (c Client) Update(ctx context.Context, team *gensql.Team) error {
	err := c.repo.TeamUpdate(ctx, team)
	if err != nil {
		return fmt.Errorf("updating team in database: %w", err)
	}

	namespace := k8s.TeamIDToNamespace(team.ID)
	err = c.manager.ApplyNamespace(ctx, core.NewNamespace(namespace, core.WithTeamNamespaceLabel()))
	if err != nil {
		return fmt.Errorf("updating k8s namespace: %w", err)
	}

	err = c.manager.ApplyServiceAccount(ctx, core.NewServiceAccount(team.ID, namespace))
	if err != nil {
		return fmt.Errorf("updating k8s service account: %w", err)
	}

	if err := c.updateGCPTeamResources(ctx, team); err != nil {
		return fmt.Errorf("updating GCP resources: %w", err)
	}

	apps, err := c.repo.ChartsForTeamGet(ctx, team.ID)
	if err != nil {
		return fmt.Errorf("getting apps for team: %w", err)
	}

	for _, app := range apps {
		switch app {
		case gensql.ChartTypeJupyterhub:
			jupyterValues := chart.JupyterConfigurableValues{
				TeamID: team.ID,
			}
			if err := c.repo.RegisterUpdateJupyterEvent(ctx, team.ID, jupyterValues); err != nil {
				return fmt.Errorf("registering Jupyter update event: %w", err)
			}
		case gensql.ChartTypeAirflow:
			airflowValues := chart.AirflowConfigurableValues{
				TeamID: team.ID,
			}
			if err := c.repo.RegisterUpdateAirflowEvent(ctx, team.ID, airflowValues); err != nil {
				return fmt.Errorf("registering Airflow update event: %w", err)
			}
		}
	}

	return nil
}

func (c Client) Delete(ctx context.Context, teamID string) error {
	team, err := c.repo.TeamGet(ctx, teamID)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("getting team from database: %w", err)
	}

	if err = c.deleteGCPTeamResources(ctx, team.ID); err != nil {
		return fmt.Errorf("deleting GCP resources: %w", err)
	}

	if err = c.manager.DeleteNamespace(ctx, k8s.TeamIDToNamespace(team.ID)); err != nil {
		return fmt.Errorf("deleting k8s namespace: %w", err)
	}

	if err = c.repo.TeamDelete(ctx, team.ID); err != nil && errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("deleting team from database: %w", err)
	}

	// Kun Airflow som har ressurser utenfor clusteret
	err = c.repo.RegisterDeleteAirflowEvent(ctx, team.ID)
	if err != nil {
		return fmt.Errorf("registering Airflow delete event: %w", err)
	}

	return nil
}
