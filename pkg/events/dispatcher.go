package events

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/team"
	"github.com/sirupsen/logrus"
)

type WorkerFunc func(context.Context, gensql.Event)

var (
	eventChan    = make(chan string, 10)
	dbQuerier    gensql.Querier
	log          *logrus.Entry
	eventContext context.Context
	teamClient   *team.Client
)

var eventWorker = map[gensql.EventType]WorkerFunc{
	gensql.EventTypeCreateTeam: func(ctx context.Context, event gensql.Event) {
		err := createTeam(ctx, event)
		if err != nil {
			log.WithError(err).Error("can't set status for event")
			return
		}
	},

	gensql.EventTypeUpdateTeam: func(ctx context.Context, event gensql.Event) {
		status := updateTeam(ctx, event)
		err := setEventStatus(event.ID, status)
		if err != nil {
			log.WithError(err).Error("can't set event status")
		}
	},
	gensql.EventTypeDeleteTeam: func(ctx context.Context, event gensql.Event) {
		err := deleteTeam(ctx, event)
		if err != nil {
			log.WithError(err).Error("can't set event status")
		}
	},
}

func setEventStatus(id uuid.UUID, status gensql.EventStatus) error {
	err := dbQuerier.EventSetStatus(eventContext, gensql.EventSetStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		log.Errorf("can't change status to %v for event(%v): %v", status, id, err)
	}

	return nil
}

func triggerDispatcher(incomingEvent string) {
	select {
	case eventChan <- incomingEvent:
	default:
	}
}

func Start(ctx context.Context, querier gensql.Querier, tClient *team.Client, logEntry *logrus.Entry) {
	log = logEntry
	dbQuerier = querier
	teamClient = tClient
	eventContext = ctx

	eventRetrievers := []func() ([]gensql.Event, error){
		func() ([]gensql.Event, error) {
			return querier.EventsGetNew(ctx)
		},
		func() ([]gensql.Event, error) {
			return querier.EventsGetOverdue(ctx)
		},
		func() ([]gensql.Event, error) {
			return querier.EventsGetPending(ctx)
		},
	}

	go func() {
		for {
			select {
			case incomingEvent := <-eventChan:
				log.Debug("Received event: ", incomingEvent)
			case <-time.Tick(1 * time.Minute):
				log.Debug("Event dispatcher run!")
			case <-ctx.Done():
				log.Debug("Context cancelled, stopping the event dispatcher.")
				return
			}

			for _, eventRetriever := range eventRetrievers {
				pickedEvents, err := eventRetriever()
				if err != nil {
					log.Errorf("Failed to fetch events: %v", err)
					continue
				}

				for _, event := range pickedEvents {
					worker, ok := eventWorker[event.EventType]
					if !ok {
						log.Errorf("No worker found for event type %v", event.EventType)
						continue
					}

					go worker(ctx, event)
				}
			}
		}
	}()
}
