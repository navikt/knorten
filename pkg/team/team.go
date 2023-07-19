package team

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/nais/knorten/pkg/logger"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

type Client struct {
	repo       *database.Repo
	k8sClient  *kubernetes.Clientset
	gcpProject string
	dryRun     bool
	log        *logrus.Entry
}

func NewClient(repo *database.Repo, gcpProject string, dryRun, inCluster bool, log *logrus.Entry) (*Client, error) {
	k8sClient, err := k8s.CreateClientset(inCluster)
	if err != nil {
		return nil, err
	}

	return &Client{
		repo:       repo,
		k8sClient:  k8sClient,
		gcpProject: gcpProject,
		dryRun:     dryRun,
		log:        log,
	}, nil
}

func (c Client) Create(ctx context.Context, team gensql.Team, log logger.Logger) bool {
	log.Infof("Creating team %v", team.ID)

	existingTeam, err := c.repo.TeamGet(ctx, team.Slug)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Errorf("sql error: %v", err)
			return true
		}
	}
	if existingTeam.Slug == team.Slug {
		log.Errorf("there already exists a team with name %v", team.Slug)
		return false
	}

	if err := c.createGCPTeamResources(ctx, team); err != nil {
		log.Errorf("failed creating gcp team resources: %v", err)
		return true
	}

	namespace := k8s.TeamIDToNamespace(team.ID)
	if err := c.createK8sNamespace(ctx, namespace); err != nil {
		log.Errorf("failed creating team namespace: %v", err)
		return true
	}

	if err := c.createK8sServiceAccount(ctx, team.ID, namespace); err != nil {
		log.Errorf("failed creating team SA: %v", err)
		return true
	}

	if err := c.repo.TeamCreate(ctx, team); err != nil {
		log.Errorf("sql error: %v", err)
		return true
	}

	log.Infof("Created %v", team.ID)
	return false
}

func (c Client) Update(ctx context.Context, team gensql.Team, log logger.Logger) bool {
	err := c.repo.TeamUpdate(ctx, team)
	if err != nil {
		log.Errorf("sql error: %v", err)
		return true
	}

	if err := c.updateGCPTeamResources(ctx, team); err != nil {
		c.log.WithError(err).Error("failed while updating gcp resources")
		return true
	}

	jupyterValues := chart.JupyterConfigurableValues{
		Slug: team.Slug,
	}
	if err := c.repo.RegisterUpdateJupyterEvent(ctx, jupyterValues); err != nil {
		c.log.WithError(err).Error("failed while registering jupyter update event")
		return true
	}

	airflowValues := chart.AirflowConfigurableValues{
		Slug: team.Slug,
	}
	if err := c.repo.RegisterUpdateAirflowEvent(ctx, airflowValues); err != nil {
		c.log.WithError(err).Error("failed while registering airflow update event")
		return true
	}

	return false
}

func (c Client) Delete(ctx context.Context, teamSlug string, log logger.Logger) bool {
	team, err := c.repo.TeamGet(ctx, teamSlug)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		log.Errorf("sql error: %v", err)
		return true
	}

	if err = c.deleteGCPTeamResources(ctx, team.ID); err != nil {
		c.log.WithError(err).Error("failed while deleting external resources")
		return true
	}

	if err = c.deleteK8sNamespace(ctx, k8s.TeamIDToNamespace(team.ID)); err != nil {
		c.log.WithError(err).Error("failed while deleting external resources")
		return true
	}

	if err = c.repo.TeamDelete(ctx, team.ID); err != nil && errors.Is(err, sql.ErrNoRows) {
		log.Errorf("sql error: %v", err)
		return true
	}

	// Kun Airflow som har ressurser utenfor clusteret
	if err := c.repo.RegisterDeleteAirflowEvent(ctx, team.ID); err != nil {
		c.log.WithError(err).Error("failed while registering airflow delete event")
		return true
	}

	return false
}
