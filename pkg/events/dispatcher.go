package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/compute"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
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
	case gensql.EventTypeCreateTeam:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			return e.createTeam(event, logger)
		}
	case gensql.EventTypeUpdateTeam:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			return e.updateTeam(event, logger)
		}
	case gensql.EventTypeCreateCompute:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			return e.createCompute(event, logger)
		}
	case gensql.EventTypeDeleteTeam,
		gensql.EventTypeDeleteCompute,
		gensql.EventTypeDeleteAirflow,
		gensql.EventTypeDeleteJupyter:
		return func(ctx context.Context, event gensql.Event, logger logger.Logger) error {
			return e.deleteEvent(event, logger)
		}
	}

	return nil
}

func (e EventHandler) deleteEvent(event gensql.Event, logger logger.Logger) error {
	var teamID string
	err := json.Unmarshal(event.Task, &teamID)
	if err != nil {
		return err
	}

	e.processWork(event, teamID, logger)

	return nil
}

func (e EventHandler) processWork(event gensql.Event, form any, logger logger.Logger) {
	var retry bool
	switch event.EventType {
	case gensql.EventTypeCreateTeam:
		retry = e.teamClient.Create(e.context, form.(gensql.Team), logger)
	case gensql.EventTypeUpdateTeam:
		retry = e.teamClient.Update(e.context, form.(gensql.Team), logger)
	case gensql.EventTypeDeleteTeam:
		retry = e.teamClient.Delete(e.context, form.(string), logger)
	case gensql.EventTypeCreateCompute:
		retry = e.computeClient.Create(e.context, form.(gensql.ComputeInstance))
	case gensql.EventTypeDeleteCompute:
		retry = e.computeClient.Delete(e.context, form.(string))
	case gensql.EventTypeDeleteAirflow:
		retry = e.chartClient.DeleteAirflow(e.context, form.(string))
	case gensql.EventTypeDeleteJupyter:
		retry = e.chartClient.DeleteJupyter(e.context, form.(string))
	}

	var err error
	if retry {
		err = e.setEventStatus(event.ID, gensql.EventStatusPending)
	} else {
		err = e.setEventStatus(event.ID, gensql.EventStatusCompleted)
	}

	if err != nil {
		e.log.WithError(err).Error("can't set status for event")
		return
	}
}

func (e EventHandler) setEventStatus(id uuid.UUID, status gensql.EventStatus) error {
	err := e.repo.EventSetStatus(e.context, id, status)
	if err != nil {
		e.log.Errorf("can't change status to %v for event(%v): %v", status, id, err)
	}

	return nil
}

func NewHandler(ctx context.Context, repo *database.Repo, gcpProject, airflowChartVersion, jupyterChartVersion string, dryRun, inCluster bool, log *logrus.Entry) (EventHandler, error) {
	teamClient, err := team.NewClient(repo, gcpProject, dryRun, inCluster, log.WithField("subsystem", "teamClient"))
	if err != nil {
		return EventHandler{}, err
	}

	chartClient, err := chart.NewClient(repo, dryRun, inCluster, gcpProject, airflowChartVersion, jupyterChartVersion, log.WithField("subsystem", "chartClient"))
	if err != nil {
		return EventHandler{}, err
	}

	return EventHandler{
		repo:          repo,
		log:           log,
		context:       ctx,
		teamClient:    teamClient,
		computeClient: compute.NewClient(repo, gcpProject, dryRun, log.WithField("subsystem", "computeClient")),
		chartClient:   chartClient,
	}, nil
}

func (e EventHandler) Run() {
	eventRetrievers := []func() ([]gensql.Event, error){
		func() ([]gensql.Event, error) {
			return e.repo.EventsGetNew(e.context)
		},
		func() ([]gensql.Event, error) {
			return e.repo.EventsGetOverdue(e.context)
		},
		func() ([]gensql.Event, error) {
			return e.repo.EventsGetPending(e.context)
		},
	}

	go func() {
		for {
			select {
			case <-time.Tick(1 * time.Minute):
				e.log.Debug("Event dispatcher run!")
			case <-e.context.Done():
				e.log.Debug("Context cancelled, stopping the event dispatcher.")
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
						err := worker(e.context, event, eventLogger)
						if err != nil {
							eventLogger.log.WithError(err).Errorf("retrieved event with invalid param: %v", err)
							err = e.setEventStatus(event.ID, gensql.EventStatusFailed)
							if err != nil {
								eventLogger.log.WithError(err).Error("can't set status for event")
							}
						}
					}()
				}
			}
		}
	}()
}
