package events

import (
	"context"
	"encoding/json"
	"time"

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

// eventWorker is a placeholder for the actual event worker function
var eventWorker = map[gensql.EventType]WorkerFunc{
	gensql.EventTypeCreateTeam: func(ctx context.Context, event gensql.Event) {
		var form team.Form
		logger := newEventLogger(event.ID)
		err := json.Unmarshal(event.Task, &form)
		if err != nil {
			log.WithField("eventType", event.EventType).WithField("eventID", event.ID).Errorf("retrieved event with invalid param: %v", err)
			err = dbQuerier.EventSetStatus(ctx, gensql.EventSetStatusParams{
				ID:     event.ID,
				Status: gensql.EventStatusFailed,
			})
			if err != nil {
				log.Errorf("can't change status to %v for %v: %v\n", gensql.EventStatusFailed, event.EventType, err)
			}
		}
		teamClient.Create(ctx, form, logger)
		err = dbQuerier.EventSetStatus(ctx, gensql.EventSetStatusParams{
			ID:     event.ID,
			Status: gensql.EventStatusCompleted,
		})
		if err != nil {
			log.Errorf("can't change status to %v for %v: %v\n", gensql.EventStatusFailed, event.EventType, err)
		}
	},
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
					log.Errorf("Failed to fetch events: %v\n", err)
					continue
				}

				for _, event := range pickedEvents {
					worker, ok := eventWorker[event.EventType]
					if !ok {
						log.Errorf("No worker found for event type %v\n", event.EventType)
						continue
					}

					go worker(ctx, event)
				}
			}
		}
	}()
}
