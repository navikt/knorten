package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/navikt/knorten/pkg/gcpapi"
	"github.com/navikt/knorten/pkg/k8s"

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
		var values helm.EventData
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

	var err error
	switch database.EventType(event.Type) {
	case database.EventTypeCreateTeam:
		t, ok := form.(*gensql.Team)
		if !ok {
			return fmt.Errorf("invalid form type for event type %v", event.Type)
		}

		logger.Infof("Creating team '%v'", t.ID)
		err = e.teamClient.Create(ctx, t)
	case database.EventTypeUpdateTeam:
		t, ok := form.(*gensql.Team)
		if !ok {
			return fmt.Errorf("invalid form type for event type %v", event.Type)
		}

		logger.Infof("Updating team '%v'", t.ID)
		err = e.teamClient.Update(ctx, t)
	case database.EventTypeDeleteTeam:
		err = e.teamClient.Delete(ctx, event.Owner)
	case database.EventTypeCreateUserGSM:
		m, ok := form.(*gensql.UserGoogleSecretManager)
		if !ok {
			return fmt.Errorf("invalid form type for event type %v", event.Type)
		}

		logger.Infof("Creating Google Secret Manager for user '%v'", m.Owner)
		err = e.userClient.CreateUserGSM(ctx, m)
	case database.EventTypeDeleteUserGSM:
		err = e.userClient.DeleteUserGSM(ctx, event.Owner)
	case database.EventTypeCreateCompute:
		i, ok := form.(*gensql.ComputeInstance)
		if !ok {
			return fmt.Errorf("invalid form type for event type %v", event.Type)
		}

		logger.Infof("Creating compute instance '%v'", i)
		err = e.userClient.CreateComputeInstance(ctx, i)
	case database.EventTypeResizeCompute:
		i, ok := form.(*gensql.ComputeInstance)
		if !ok {
			return fmt.Errorf("invalid form type for event type %v", event.Type)
		}

		logger.Infof("Resizing disk for compute instance '%v'", i.Owner)
		err = e.userClient.ResizeComputeInstanceDisk(ctx, i)
	case database.EventTypeDeleteCompute:
		err = e.userClient.DeleteComputeInstance(ctx, event.Owner)
	case database.EventTypeCreateAirflow, database.EventTypeUpdateAirflow:
		v, ok := form.(*chart.AirflowConfigurableValues)
		if !ok {
			return fmt.Errorf("invalid form type for event type %v", event.Type)
		}

		logger.Infof("Syncing Airflow for team '%v'", v.TeamID)
		err = e.chartClient.SyncAirflow(ctx, v)
	case database.EventTypeDeleteAirflow:
		err = e.chartClient.DeleteAirflow(ctx, event.Owner)
	case database.EventTypeCreateJupyter, database.EventTypeUpdateJupyter:
		v, ok := form.(*chart.JupyterConfigurableValues)
		if !ok {
			return fmt.Errorf("invalid form type for event type %v", event.Type)
		}

		logger.Infof("Syncing Jupyter for team '%v'", v.TeamID)
		err = e.chartClient.SyncJupyter(ctx, v)
	case database.EventTypeDeleteJupyter:
		err = e.chartClient.DeleteJupyter(ctx, event.Owner)
	case database.EventTypeHelmRolloutJupyter, database.EventTypeHelmRolloutAirflow:
		d, ok := form.(*helm.EventData)
		if !ok {
			return fmt.Errorf("invalid form type for event type %v", event.Type)
		}

		logger.Infof("Rolling out helm chart for team '%v'", d.TeamID)
		err = e.helmClient.InstallOrUpgrade(ctx, d)
	case database.EventTypeHelmRollbackJupyter, database.EventTypeHelmRollbackAirflow:
		d, ok := form.(*helm.EventData)
		if !ok {
			return fmt.Errorf("invalid form type for event type %v", event.Type)
		}

		logger.Infof("Rolling back helm chart for team '%v'", d.TeamID)
		err = e.helmClient.Rollback(ctx, d)
	case database.EventTypeHelmUninstallJupyter, database.EventTypeHelmUninstallAirflow:
		d, ok := form.(*helm.EventData)
		if !ok {
			return fmt.Errorf("invalid form type for event type %v", event.Type)
		}

		logger.Infof("Uninstalling helm chart for team '%v'", d.TeamID)
		err = e.helmClient.Uninstall(ctx, d)
	}

	if err != nil {
		logger.WithError(err).Error("failed processing event")
		return fmt.Errorf("failed processing event: %w", err)
	}

	return e.repo.EventSetStatus(e.context, event.ID, database.EventStatusCompleted)
}

func NewHandler(
	ctx context.Context,
	repo *database.Repo,
	azureClient *auth.Azure,
	mngr k8s.Manager,
	saBinder gcpapi.ServiceAccountPolicyBinder,
	saChecker gcpapi.ServiceAccountChecker,
	client *helm.Client,
	gcpProject, gcpRegion, gcpZone, airflowChartVersion, jupyterChartVersion, topLevelDomain string,
	dryRun bool,
	log *logrus.Entry,
) (EventHandler, error) {
	teamClient, err := team.NewClient(repo, mngr, gcpProject, gcpRegion, dryRun)
	if err != nil {
		return EventHandler{}, err
	}

	chartClient, err := chart.NewClient(
		repo,
		azureClient,
		mngr,
		saBinder,
		saChecker,
		dryRun,
		airflowChartVersion,
		jupyterChartVersion,
		gcpProject,
		gcpRegion,
		topLevelDomain,
	)
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
		helmClient:  client,
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
