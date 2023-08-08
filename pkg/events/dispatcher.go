package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nais/knorten/pkg/api/auth"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/compute"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/leaderelection"
	"github.com/nais/knorten/pkg/logger"
	"github.com/nais/knorten/pkg/team"
	"github.com/sirupsen/logrus"
)

type EventHandler struct {
	repo          *database.Repo
	log           *logrus.Entry
	context       context.Context
	teamClient    *team.Client
	computeClient *compute.Client
	chartClient   *chart.Client
}

type workerFunc func(context.Context, gensql.Event, logger.Logger) error

func (e EventHandler) distributeWork(eventType gensql.EventType) workerFunc {
	switch eventType {
	case gensql.EventTypeCreateTeam,
		gensql.EventTypeUpdateTeam:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			var team gensql.Team
			return e.processWork(event, logger, &team)
		}
	case gensql.EventTypeCreateCompute:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			var instance gensql.ComputeInstance
			return e.processWork(event, logger, &instance)
		}
	case gensql.EventTypeCreateAirflow,
		gensql.EventTypeUpdateAirflow:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			var values chart.AirflowConfigurableValues
			return e.processWork(event, logger, &values)
		}
	case gensql.EventTypeCreateJupyter,
		gensql.EventTypeUpdateJupyter:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			var values chart.JupyterConfigurableValues
			return e.processWork(event, logger, &values)
		}
	case gensql.EventTypeDeleteTeam,
		gensql.EventTypeDeleteCompute,
		gensql.EventTypeDeleteAirflow,
		gensql.EventTypeDeleteJupyter:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			var value string
			return e.processWork(event, logger, &value)
		}
	}

	return nil
}

func (e EventHandler) processWork(event gensql.Event, logger logger.Logger, form any) error {
	if err := json.Unmarshal(event.Payload, &form); err != nil {
		if err := e.repo.EventSetStatus(e.context, event.ID, gensql.EventStatusFailed); err != nil {
			return err
		}

		return err
	}

	err := e.repo.EventSetStatus(e.context, event.ID, gensql.EventStatusProcessing)
	if err != nil {
		return err
	}

	var retry bool
	switch event.EventType {
	case gensql.EventTypeCreateTeam:
		retry = e.teamClient.Create(e.context, *form.(*gensql.Team), logger)
	case gensql.EventTypeUpdateTeam:
		retry = e.teamClient.Update(e.context, *form.(*gensql.Team), logger)
	case gensql.EventTypeDeleteTeam:
		retry = e.teamClient.Delete(e.context, *form.(*string), logger)
	case gensql.EventTypeCreateCompute:
		retry = e.computeClient.Create(e.context, *form.(*gensql.ComputeInstance), logger)
	case gensql.EventTypeDeleteCompute:
		retry = e.computeClient.Delete(e.context, *form.(*string), logger)
	case gensql.EventTypeCreateAirflow,
		gensql.EventTypeUpdateAirflow:
		retry = e.chartClient.SyncAirflow(e.context, *form.(*chart.AirflowConfigurableValues), logger)
	case gensql.EventTypeDeleteAirflow:
		retry = e.chartClient.DeleteAirflow(e.context, *form.(*string), logger)
	case gensql.EventTypeCreateJupyter,
		gensql.EventTypeUpdateJupyter:
		retry = e.chartClient.SyncJupyter(e.context, *form.(*chart.JupyterConfigurableValues), logger)
	case gensql.EventTypeDeleteJupyter:
		retry = e.chartClient.DeleteJupyter(e.context, *form.(*string), logger)
	}

	if retry {
		return e.repo.EventSetPendingStatus(e.context, event.ID)
	}

	return e.repo.EventSetStatus(e.context, event.ID, gensql.EventStatusCompleted)
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
		repo:          repo,
		log:           log,
		context:       ctx,
		teamClient:    teamClient,
		computeClient: compute.NewClient(repo, gcpProject, gcpZone, dryRun),
		chartClient:   chartClient,
	}, nil
}

func (e EventHandler) Run(tickDuration time.Duration) {
	eventRetrievers := []func() ([]gensql.Event, error){
		func() ([]gensql.Event, error) {
			return e.repo.EventsGetNew(e.context)
		},
		func() ([]gensql.Event, error) {
			return e.repo.EventsGetOverdue(e.context)
		},
	}

	go func() {
		for {
			select {
			case <-time.NewTicker(tickDuration).C:
				e.log.Debug("Event dispatcher run!")
			case <-e.context.Done():
				e.log.Debug("Context cancelled, stopping the event dispatcher.")
				return
			}

			isLeader, err := leaderelection.IsLeader()
			if err != nil {
				e.log.WithError(err).Error("leader election check")
				return
			}
			if !isLeader {
				return
			}

			for _, eventRetriever := range eventRetrievers {
				pickedEvents, err := eventRetriever()
				if err != nil {
					e.log.Errorf("Failed to fetch events: %v", err)
					continue
				}

				for _, event := range pickedEvents {
					worker := e.distributeWork(event.EventType)
					if worker == nil {
						e.log.WithField("eventID", event.ID).Errorf("No worker found for event type %v", event.EventType)
						continue
					}

					eventLogger := newEventLogger(e.context, e.log, e.repo, event)
					eventLogger.log.Infof("Dispatching event '%v'", event.EventType)
					event := event
					go func() {
						if err := worker(e.context, event, eventLogger); err != nil {
							eventLogger.log.WithError(err).Error("failed processing event")
							if event.RetryCount > 5 {
								eventLogger.log.Error("event reached max retries")
								if err := e.repo.EventSetStatus(e.context, event.ID, gensql.EventStatusFailed); err != nil {
									eventLogger.log.WithError(err).Error("failed setting event status to 'failed'")
								}
							}
						}
					}()
				}
			}
		}
	}()
}
