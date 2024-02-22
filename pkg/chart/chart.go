package chart

import (
	"context"
	"fmt"
	"github.com/navikt/knorten/pkg/api/auth"
	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/gcpapi"
	"github.com/navikt/knorten/pkg/helm"
	"github.com/navikt/knorten/pkg/k8s"
	"github.com/navikt/knorten/pkg/logger"
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

func (c Client) SyncJupyter(ctx context.Context, values JupyterConfigurableValues, log logger.Logger) bool {
	log.Info("Syncing Jupyter")

	if err := c.syncJupyter(ctx, values, log); err != nil {
		log.Info("Failed syncing Jupyter")
		return true
	}

	if err := c.registerJupyterHelmEvent(ctx, values.TeamID, database.EventTypeHelmRolloutJupyter, log); err != nil {
		log.Info("Failed creating rollout jupyter helm event")
		return true
	}

	log.Info("Successfully synced Jupyter")
	return false
}

func (c Client) DeleteJupyter(ctx context.Context, teamID string, log logger.Logger) bool {
	log.Info("Deleting Jupyter")

	if err := c.deleteJupyter(ctx, teamID, log); err != nil {
		log.Info("Failed deleting Jupyter")
		return true
	}

	if err := c.registerJupyterHelmEvent(ctx, teamID, database.EventTypeHelmUninstallJupyter, log); err != nil {
		log.Info("Failed creating uninstall jupyter helm event")
		return true
	}

	log.Info("Successfully deleted Jupyter")
	return false
}

func (c Client) SyncAirflow(ctx context.Context, values AirflowConfigurableValues, log logger.Logger) bool {
	log.Info("Syncing Airflow")

	if err := c.syncAirflow(ctx, values, log); err != nil {
		log.Info("Failed syncing Airflow")
		return true
	}

	if err := c.registerAirflowHelmEvent(ctx, values.TeamID, database.EventTypeHelmRolloutAirflow, log); err != nil {
		log.Info("Failed creating rollout airflow helm event")
		return true
	}

	log.Info("Successfully synced Airflow")
	return false
}

func (c Client) DeleteAirflow(ctx context.Context, teamID string, log logger.Logger) bool {
	log.Info("Deleting Airflow")

	if err := c.deleteAirflow(ctx, teamID, log); err != nil {
		log.Info("Failed deleting Airflow")
		return true
	}

	if err := c.registerAirflowHelmEvent(ctx, teamID, database.EventTypeHelmUninstallAirflow, log); err != nil {
		log.Info("Failed creating uninstall airflow helm event")
		return true
	}

	log.Info("Successfully deleted Airflow")
	return false
}

func (c Client) registerHelmEvent(ctx context.Context, eventType database.EventType, teamID string, helmEventData helm.HelmEventData, logger logger.Logger) error {
	switch eventType {
	case database.EventTypeHelmRolloutJupyter:
		if err := c.repo.RegisterHelmRolloutJupyterEvent(ctx, teamID, helmEventData); err != nil {
			logger.WithError(err).Error("registering rollout jupyter event failed")
			return err
		}
	case database.EventTypeHelmUninstallJupyter:
		if err := c.repo.RegisterHelmUninstallJupyterEvent(ctx, teamID, helmEventData); err != nil {
			logger.WithError(err).Error("registering uninstall jupyter event failed")
			return err
		}
	case database.EventTypeHelmRolloutAirflow:
		if err := c.repo.RegisterHelmRolloutAirflowEvent(ctx, teamID, helmEventData); err != nil {
			logger.WithError(err).Error("registering rollout airflow event failed")
			return err
		}
	case database.EventTypeHelmUninstallAirflow:
		if err := c.repo.RegisterHelmUninstallAirflowEvent(ctx, teamID, helmEventData); err != nil {
			logger.WithError(err).Error("registering uninstall airflow event failed")
			return err
		}
	default:
		return fmt.Errorf("eventType %v not supported", eventType)
	}

	return nil
}
