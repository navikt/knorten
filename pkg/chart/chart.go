package chart

import (
	"context"
	"fmt"
	"github.com/navikt/knorten/pkg/api/auth"
	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/gcpapi"
	"github.com/navikt/knorten/pkg/helm"
	"github.com/navikt/knorten/pkg/k8s"
)

type Client struct {
	repo                *database.Repo
	manager             k8s.Manager
	saBinder            gcpapi.ServiceAccountPolicyBinder
	saChecker           gcpapi.ServiceAccountChecker
	azureClient         *auth.Azure
	dryRun              bool
	chartVersionAirflow string
	chartVersionJupyter string
	gcpProject          string
	gcpRegion           string
}

func NewClient(
	repo *database.Repo,
	azureClient *auth.Azure,
	mngr k8s.Manager,
	saBinder gcpapi.ServiceAccountPolicyBinder,
	saChecker gcpapi.ServiceAccountChecker,
	dryRun bool,
	airflowChartVersion, jupyterChartVersion, gcpProject, gcpRegion string,
) (*Client, error) {
	return &Client{
		repo:                repo,
		manager:             mngr,
		saBinder:            saBinder,
		saChecker:           saChecker,
		azureClient:         azureClient,
		dryRun:              dryRun,
		chartVersionAirflow: airflowChartVersion,
		chartVersionJupyter: jupyterChartVersion,
		gcpProject:          gcpProject,
		gcpRegion:           gcpRegion,
	}, nil
}

func (c Client) SyncJupyter(ctx context.Context, values *JupyterConfigurableValues) error {
	err := c.syncJupyter(ctx, values)
	if err != nil {
		return fmt.Errorf("syncing jupyter: %w", err)
	}

	err = c.registerJupyterHelmEvent(ctx, values.TeamID, database.EventTypeHelmRolloutJupyter)
	if err != nil {
		return fmt.Errorf("registering jupyter helm event: %w", err)
	}

	return nil
}

func (c Client) DeleteJupyter(ctx context.Context, teamID string) error {
	err := c.deleteJupyter(ctx, teamID)
	if err != nil {
		return fmt.Errorf("deleting jupyter: %w", err)
	}

	err = c.registerJupyterHelmEvent(ctx, teamID, database.EventTypeHelmUninstallJupyter)
	if err != nil {
		return fmt.Errorf("registering jupyter helm event: %w", err)
	}

	return nil
}

func (c Client) SyncAirflow(ctx context.Context, values *AirflowConfigurableValues) error {
	err := c.syncAirflow(ctx, values)
	if err != nil {
		return fmt.Errorf("syncing airflow: %w", err)
	}

	err = c.registerAirflowHelmEvent(ctx, values.TeamID, database.EventTypeHelmRolloutAirflow)
	if err != nil {
		return fmt.Errorf("registering airflow helm event: %w", err)
	}

	return nil
}

func (c Client) DeleteAirflow(ctx context.Context, teamID string) error {
	err := c.deleteAirflow(ctx, teamID)
	if err != nil {
		return fmt.Errorf("deleting airflow: %w", err)
	}

	err = c.registerAirflowHelmEvent(ctx, teamID, database.EventTypeHelmUninstallAirflow)
	if err != nil {
		return fmt.Errorf("registering airflow helm event: %w", err)
	}

	return nil
}

func (c Client) registerHelmEvent(ctx context.Context, eventType database.EventType, teamID string, helmEventData helm.EventData) error {
	switch eventType {
	case database.EventTypeHelmRolloutJupyter:
		if err := c.repo.RegisterHelmRolloutJupyterEvent(ctx, teamID, helmEventData); err != nil {
			return err
		}
	case database.EventTypeHelmUninstallJupyter:
		if err := c.repo.RegisterHelmUninstallJupyterEvent(ctx, teamID, helmEventData); err != nil {
			return err
		}
	case database.EventTypeHelmRolloutAirflow:
		if err := c.repo.RegisterHelmRolloutAirflowEvent(ctx, teamID, helmEventData); err != nil {
			return err
		}
	case database.EventTypeHelmUninstallAirflow:
		if err := c.repo.RegisterHelmUninstallAirflowEvent(ctx, teamID, helmEventData); err != nil {
			return err
		}
	default:
		return fmt.Errorf("eventType %v not supported", eventType)
	}

	return nil
}
