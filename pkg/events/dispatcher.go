package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nais/knorten/pkg/api/auth"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
	"github.com/nais/knorten/pkg/leaderelection"
	"github.com/nais/knorten/pkg/logger"
	"github.com/nais/knorten/pkg/team"
	"github.com/nais/knorten/pkg/user"
	"github.com/sirupsen/logrus"
)

type EventHandler struct {
	repo        database.Repository
	log         *logrus.Entry
	context     context.Context
	teamClient  teamClient
	userClient  userClient
	chartClient chartClient
	helmClient  helm.Client
}

type workerFunc func(context.Context, gensql.Event, logger.Logger) error

func (e EventHandler) distributeWork(eventType database.EventType) workerFunc {
	switch eventType {
	case database.EventTypeCreateTeam,
		database.EventTypeUpdateTeam:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			var team gensql.Team
			return e.processWork(event, logger, &team)
		}
	case database.EventTypeCreateUserGSM:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			var manager gensql.UserGoogleSecretManager
			return e.processWork(event, logger, &manager)
		}
	case database.EventTypeCreateCompute:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			var instance gensql.ComputeInstance
			return e.processWork(event, logger, &instance)
		}
	case database.EventTypeCreateAirflow,
		database.EventTypeUpdateAirflow:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			var values chart.AirflowConfigurableValues
			return e.processWork(event, logger, &values)
		}
	case database.EventTypeCreateJupyter,
		database.EventTypeUpdateJupyter:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			var values chart.JupyterConfigurableValues
			return e.processWork(event, logger, &values)
		}
	case database.EventTypeDeleteTeam,
		database.EventTypeDeleteUserGSM,
		database.EventTypeDeleteCompute,
		database.EventTypeDeleteAirflow,
		database.EventTypeDeleteJupyter:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			return e.processWork(event, logger, nil)
		}
	case database.EventTypeHelmInstall,
		database.EventTypeHelmUpgrade,
		database.EventTypeHelmRollback,
		database.EventTypeHelmUninstall:
		var values database.HelmEvent
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			return e.processWork(event, logger, &values)
		}
	}

	return nil
}

func (e EventHandler) processWork(event gensql.Event, logger logger.Logger, form any) error {
	if err := json.Unmarshal(event.Payload, &form); err != nil {
		if err := e.repo.EventSetStatus(e.context, event.ID, database.EventStatusFailed); err != nil {
			return err
		}
		return err
	}

	err := e.repo.EventSetStatus(e.context, event.ID, database.EventStatusProcessing)
	if err != nil {
		return err
	}

	var retry bool
	switch database.EventType(event.Type) {
	case database.EventTypeCreateTeam:
		retry = e.teamClient.Create(e.context, *form.(*gensql.Team), logger)
	case database.EventTypeUpdateTeam:
		retry = e.teamClient.Update(e.context, *form.(*gensql.Team), logger)
	case database.EventTypeDeleteTeam:
		retry = e.teamClient.Delete(e.context, event.Owner, logger)
	case database.EventTypeCreateUserGSM:
		retry = e.userClient.CreateUserGSM(e.context, *form.(*gensql.UserGoogleSecretManager), logger)
	case database.EventTypeDeleteUserGSM:
		retry = e.userClient.DeleteUserGSM(e.context, event.Owner, logger)
	case database.EventTypeCreateCompute:
		retry = e.userClient.CreateComputeInstance(e.context, *form.(*gensql.ComputeInstance), logger)
	case database.EventTypeDeleteCompute:
		retry = e.userClient.DeleteComputeInstance(e.context, event.Owner, logger)
	case database.EventTypeCreateAirflow,
		database.EventTypeUpdateAirflow:
		retry = e.chartClient.SyncAirflow(e.context, *form.(*chart.AirflowConfigurableValues), logger)
	case database.EventTypeDeleteAirflow:
		retry = e.chartClient.DeleteAirflow(e.context, event.Owner, logger)
	case database.EventTypeCreateJupyter,
		database.EventTypeUpdateJupyter:
		retry = e.chartClient.SyncJupyter(e.context, *form.(*chart.JupyterConfigurableValues), logger)
	case database.EventTypeDeleteJupyter:
		retry = e.chartClient.DeleteJupyter(e.context, event.Owner, logger)
	case database.EventTypeHelmInstall,
		database.EventTypeHelmUpgrade:
		helmEvent := *form.(*database.HelmEvent)
		go e.helmClient.HelmTimeoutWatcher(e.context, helmEvent, logger)
		if err := e.helmClient.InstallOrUpgrade(e.context, helmEvent, logger); err != nil {
			logger.WithError(err).Error("helm install or upgrade failed")
			if err := e.repo.RegisterHelmRollbackEvent(e.context, helmEvent); err != nil {
				logger.WithError(err).Error("registering helm rollback event")
			}
			if err := e.repo.EventSetStatus(e.context, event.ID, database.EventStatusFailed); err != nil {
				logger.WithError(err).Error("setting event status to failed")
			}
			return nil
		}
	case database.EventTypeHelmRollback:
		retry, err = e.helmClient.Rollback(e.context, *form.(*database.HelmEvent), logger)
		if err != nil {
			if err := e.repo.EventSetStatus(e.context, event.ID, database.EventStatusFailed); err != nil {
				logger.WithError(err).Error("setting event status to failed")
			}
		}
	case database.EventTypeHelmUninstall:
		retry = e.helmClient.Uninstall(e.context, *form.(*database.HelmEvent), logger)
	}

	if retry {
		return e.repo.EventSetPendingStatus(e.context, event.ID)
	}

	return e.repo.EventSetStatus(e.context, event.ID, database.EventStatusCompleted)
}

func NewHandler(ctx context.Context, repo *database.Repo, azureClient *auth.Azure, gcpProject, gcpRegion, gcpZone, airflowChartVersion, jupyterChartVersion string, dryRun, inCluster bool, log *logrus.Entry) (EventHandler, error) {
	teamClient, err := team.NewClient(repo, gcpProject, gcpRegion, dryRun, inCluster)
	if err != nil {
		return EventHandler{}, err
	}

	chartClient, err := chart.NewClient(repo, azureClient, dryRun, inCluster, airflowChartVersion, jupyterChartVersion, gcpProject, gcpRegion)
	if err != nil {
		return EventHandler{}, err
	}

	return EventHandler{
		repo:        repo,
		log:         log,
		context:     ctx,
		teamClient:  teamClient,
		userClient:  user.NewClient(repo, gcpProject, gcpRegion, gcpZone, dryRun),
		chartClient: chartClient,
		helmClient:  helm.NewClient(dryRun, repo),
	}, nil
}

func (e EventHandler) Run(tickDuration time.Duration) {
	go func() {
		var cancelFuncs []context.CancelFunc
		for {
			select {
			case <-time.NewTicker(tickDuration).C:
				e.log.Debug("Event dispatcher run!")
			case <-e.context.Done():
				e.log.Debug("Context cancelled, stopping the event dispatcher.")
				for _, cancelFunc := range cancelFuncs {
					cancelFunc()
				}
				return
			}

			isLeader, err := leaderelection.IsLeader()
			if err != nil {
				e.log.WithError(err).Error("leader election check")
				continue
			}
			if !isLeader {
				continue
			}

			events, err := e.repo.DispatchableEventsGet(e.context)
			if err != nil {
				e.log.WithError(err).Error("failed to fetch events")
				continue
			}

			for _, event := range events {
				worker := e.distributeWork(database.EventType(event.Type))
				if worker == nil {
					e.log.WithField("eventID", event.ID).Errorf("No worker found for event type %v", event.Type)
					continue
				}

				eventLogger := newEventLogger(e.context, e.log, e.repo, event)
				eventLogger.log.Infof("Dispatching event '%v'", event.Type)
				event := event
				go func() {
					deadline, err := time.ParseDuration(event.Deadline)
					if err != nil {
						eventLogger.log.WithError(err).Error("failed parsing event deadline")
						return
					}

					ctx, cancelFunc := context.WithTimeout(e.context, deadline)
					cancelFuncs = append(cancelFuncs, cancelFunc)

					if err := worker(ctx, event, eventLogger); err != nil {
						eventLogger.log.WithError(err).Error("failed processing event")
						if event.RetryCount > 5 {
							eventLogger.log.Error("event reached max retries")
							if err := e.repo.EventSetStatus(e.context, event.ID, database.EventStatusFailed); err != nil {
								eventLogger.log.WithError(err).Error("failed setting event status to 'failed'")
							}
						}
					}
				}()
			}
		}
	}()
}
