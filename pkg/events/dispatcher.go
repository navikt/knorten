package events

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/navikt/knorten/pkg/k8s"
	"time"

	"github.com/navikt/knorten/pkg/api/auth"
	"github.com/navikt/knorten/pkg/chart"
	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/helm"
	"github.com/navikt/knorten/pkg/leaderelection"
	"github.com/navikt/knorten/pkg/logger"
	"github.com/navikt/knorten/pkg/team"
	"github.com/navikt/knorten/pkg/user"
	"github.com/sirupsen/logrus"
)

type EventHandler struct {
	repo        database.Repository
	log         *logrus.Entry
	context     context.Context
	teamClient  teamClient
	userClient  userClient
	chartClient chartClient
	helmClient  helmClient
}

const (
	maxConcurrentEventsHandled = 5
)

type workerFunc func(context.Context, gensql.Event, logger.Logger) error

func (e EventHandler) distributeWork(eventType database.EventType) workerFunc {
	switch eventType {
	case database.EventTypeCreateTeam,
		database.EventTypeUpdateTeam:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			var team gensql.Team
			return e.processWork(ctx, event, logger, &team)
		}
	case database.EventTypeCreateUserGSM:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			var manager gensql.UserGoogleSecretManager
			return e.processWork(ctx, event, logger, &manager)
		}
	case database.EventTypeCreateCompute,
		database.EventTypeResizeCompute:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			var instance gensql.ComputeInstance
			return e.processWork(ctx, event, logger, &instance)
		}
	case database.EventTypeCreateAirflow,
		database.EventTypeUpdateAirflow:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			var values chart.AirflowConfigurableValues
			return e.processWork(ctx, event, logger, &values)
		}
	case database.EventTypeCreateJupyter,
		database.EventTypeUpdateJupyter:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			var values chart.JupyterConfigurableValues
			return e.processWork(ctx, event, logger, &values)
		}
	case database.EventTypeDeleteTeam,
		database.EventTypeDeleteUserGSM,
		database.EventTypeDeleteCompute,
		database.EventTypeDeleteAirflow,
		database.EventTypeDeleteJupyter:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			return e.processWork(ctx, event, logger, nil)
		}
	case database.EventTypeHelmRolloutJupyter,
		database.EventTypeHelmRollbackJupyter,
		database.EventTypeHelmUninstallJupyter,
		database.EventTypeHelmRolloutAirflow,
		database.EventTypeHelmRollbackAirflow,
		database.EventTypeHelmUninstallAirflow:
		var values helm.HelmEventData
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			return e.processWork(ctx, event, logger, &values)
		}
	}

	return nil
}

func (e EventHandler) processWork(ctx context.Context, event gensql.Event, logger logger.Logger, form any) error {
	if err := json.Unmarshal(event.Payload, &form); err != nil {
		if err := e.repo.EventSetStatus(e.context, event.ID, database.EventStatusFailed); err != nil {
			return err
		}
		return err
	}

	if err := e.repo.EventSetStatus(e.context, event.ID, database.EventStatusProcessing); err != nil {
		return err
	}

	var retry bool
	var err error
	switch database.EventType(event.Type) {
	case database.EventTypeCreateTeam:
		retry = e.teamClient.Create(ctx, *form.(*gensql.Team), logger)
	case database.EventTypeUpdateTeam:
		retry = e.teamClient.Update(ctx, *form.(*gensql.Team), logger)
	case database.EventTypeDeleteTeam:
		retry = e.teamClient.Delete(ctx, event.Owner, logger)
	case database.EventTypeCreateUserGSM:
		retry = e.userClient.CreateUserGSM(ctx, *form.(*gensql.UserGoogleSecretManager), logger)
	case database.EventTypeDeleteUserGSM:
		retry = e.userClient.DeleteUserGSM(ctx, event.Owner, logger)
	case database.EventTypeCreateCompute:
		retry = e.userClient.CreateComputeInstance(ctx, *form.(*gensql.ComputeInstance), logger)
	case database.EventTypeResizeCompute:
		retry = e.userClient.ResizeComputeInstanceDisk(ctx, *form.(*gensql.ComputeInstance), logger)
	case database.EventTypeDeleteCompute:
		retry = e.userClient.DeleteComputeInstance(ctx, event.Owner, logger)
	case database.EventTypeCreateAirflow,
		database.EventTypeUpdateAirflow:
		retry = e.chartClient.SyncAirflow(ctx, *form.(*chart.AirflowConfigurableValues), logger)
	case database.EventTypeDeleteAirflow:
		retry = e.chartClient.DeleteAirflow(ctx, event.Owner, logger)
	case database.EventTypeCreateJupyter,
		database.EventTypeUpdateJupyter:
		retry = e.chartClient.SyncJupyter(ctx, *form.(*chart.JupyterConfigurableValues), logger)
	case database.EventTypeDeleteJupyter:
		retry = e.chartClient.DeleteJupyter(ctx, event.Owner, logger)
	case database.EventTypeHelmRolloutJupyter,
		database.EventTypeHelmRolloutAirflow:
		err = e.helmClient.InstallOrUpgrade(ctx, *form.(*helm.HelmEventData), logger)
	case database.EventTypeHelmRollbackJupyter,
		database.EventTypeHelmRollbackAirflow:
		retry, err = e.helmClient.Rollback(ctx, *form.(*helm.HelmEventData), logger)
	case database.EventTypeHelmUninstallJupyter,
		database.EventTypeHelmUninstallAirflow:
		retry = e.helmClient.Uninstall(ctx, *form.(*helm.HelmEventData), logger)
	}

	if err != nil {
		return e.repo.EventSetStatus(e.context, event.ID, database.EventStatusFailed)
	}

	if retry {
		return fmt.Errorf("event %v failed", event.ID)
	}

	return e.repo.EventSetStatus(e.context, event.ID, database.EventStatusCompleted)
}

func NewHandler(ctx context.Context, repo *database.Repo, azureClient *auth.Azure, mngr k8s.Manager, gcpProject, gcpRegion, gcpZone, airflowChartVersion, jupyterChartVersion string, dryRun, inCluster bool, log *logrus.Entry) (EventHandler, error) {
	teamClient, err := team.NewClient(repo, mngr, gcpProject, gcpRegion, dryRun, inCluster)
	if err != nil {
		return EventHandler{}, err
	}

	chartClient, err := chart.NewClient(repo, azureClient, mngr, dryRun, airflowChartVersion, jupyterChartVersion, gcpProject, gcpRegion)
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
	eventQueue := make(chan struct{}, maxConcurrentEventsHandled)

	var isLeader bool
	var err error
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

			isLeader, err = e.isNewLeader(isLeader)
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
				eventQueue <- struct{}{}
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
						eventLogger.log.WithError(err).Info("failed processing event")
						if event.RetryCount > 5 {
							eventLogger.log.WithError(err).Error("failed processing event, reached max retries")
							if err := e.repo.EventSetStatus(e.context, event.ID, database.EventStatusFailed); err != nil {
								eventLogger.log.WithError(err).Error("failed setting event status to 'failed'")
							}
						} else {
							if err := e.repo.EventIncrementRetryCount(e.context, event.ID); err != nil {
								eventLogger.log.WithError(err).Errorf("failed to increment retry count for event %v on error", event.ID)
							}
							select {
							case <-ctx.Done():
								eventLogger.log.WithError(err).Info("failed processing event, deadline reached")
								if err := e.repo.EventSetStatus(e.context, event.ID, database.EventStatusDeadlineReached); err != nil {
									eventLogger.log.WithError(err).Error("failed setting event status to 'deadline_reached'")
								}
							default:
								if err := e.repo.EventSetStatus(e.context, event.ID, database.EventStatusPending); err != nil {
									eventLogger.log.WithError(err).Error("failed setting event status to 'pending'")
								}
							}
						}
					}
					<-eventQueue
				}()
			}
		}
	}()
}

func (e EventHandler) isNewLeader(currentLeaderStatus bool) (bool, error) {
	isLeader, err := leaderelection.IsLeader()
	if err != nil {
		return currentLeaderStatus, err
	}

	if isLeader != currentLeaderStatus {
		if err := e.repo.EventsReset(e.context); err != nil {
			e.log.WithError(err).Error("failed to reset events on new leader")
			return isLeader, err
		}
	}

	return isLeader, nil
}
