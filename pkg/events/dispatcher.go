package events

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/compute"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/team"
	"github.com/sirupsen/logrus"
)

type eventHandler struct {
	repo          *database.Repo
	log           *logrus.Entry
	context       context.Context
	teamClient    *team.Client
	computeClient *compute.Client
}

type workerFunc func(context.Context, gensql.Event)

func (e eventHandler) distributeWork(eventType gensql.EventType) workerFunc {
	switch eventType {
	case gensql.EventTypeCreateTeam:
		return func(ctx context.Context, event gensql.Event) {
			err := e.createTeam(event)
			if err != nil {
				e.log.WithError(err).Error("can't set status for event")
				return
			}
		}

	case gensql.EventTypeUpdateTeam:
		return func(ctx context.Context, event gensql.Event) {
			status := e.updateTeam(event)
			err := e.setEventStatus(event.ID, status)
			if err != nil {
				e.log.WithError(err).Error("can't set event status")
			}
		}
	case gensql.EventTypeDeleteTeam:
		return func(ctx context.Context, event gensql.Event) {
			err := e.deleteTeam(event)
			if err != nil {
				e.log.WithError(err).Error("can't set event status")
			}
		}
	case gensql.EventTypeCreateCompute:
		return func(ctx context.Context, event gensql.Event) {
			err := e.createCompute(event)
			if err != nil {
				e.log.WithError(err).Error("can't set event status")
			}
		}
	case gensql.EventTypeDeleteCompute:
		return func(ctx context.Context, event gensql.Event) {
			err := e.deleteCompute(event)
			if err != nil {
				e.log.WithError(err).Error("can't set event status")
			}
		}
	}

	return nil
}

func (e eventHandler) setEventStatus(id uuid.UUID, status gensql.EventStatus) error {
	err := e.repo.EventSetStatus(e.context, id, status)
	if err != nil {
		e.log.Errorf("can't change status to %v for event(%v): %v", status, id, err)
	}

	return nil
}

func Start(ctx context.Context, repo *database.Repo, gcpProject string, dryRun, inCluster bool, log *logrus.Entry) error {
	teamClient, err := team.NewClient(repo, gcpProject, dryRun, inCluster, log.WithField("subsystem", "teamClient"))
	if err != nil {
		return err
	}

	handler := eventHandler{
		repo:          repo,
		log:           log,
		context:       ctx,
		teamClient:    teamClient,
		computeClient: compute.NewClient(repo, gcpProject, dryRun, log.WithField("subsystem", "computeClient")),
	}

	eventRetrievers := []func() ([]gensql.Event, error){
		func() ([]gensql.Event, error) {
			return handler.repo.EventsGetNew(ctx)
		},
		func() ([]gensql.Event, error) {
			return handler.repo.EventsGetOverdue(ctx)
		},
		func() ([]gensql.Event, error) {
			return handler.repo.EventsGetPending(ctx)
		},
	}

	go func() {
		for {
			select {
			case <-time.Tick(1 * time.Minute):
				handler.log.Debug("Event dispatcher run!")
			case <-ctx.Done():
				handler.log.Debug("Context cancelled, stopping the event dispatcher.")
				return
			}

			for _, eventRetriever := range eventRetrievers {
				pickedEvents, err := eventRetriever()
				if err != nil {
					handler.log.Errorf("Failed to fetch events: %v", err)
					continue
				}

				for _, event := range pickedEvents {
					worker := handler.distributeWork(event.EventType)
					if worker == nil {
						handler.log.WithField("eventID", event.ID).Errorf("No worker found for event type %v", event.EventType)
						continue
					}

					handler.log.WithField("eventID", event.ID).Infof("Dispatching event '%v'", event.EventType)
					go worker(ctx, event)
				}
			}
		}
	}()

	return nil
}
