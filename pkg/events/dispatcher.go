package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/team"
	"github.com/sirupsen/logrus"
)

type WorkerFunc func(context.Context, uuid.UUID, any)

var eventChan = make(chan string, 10)
var dbQuerier gensql.Querier
var log *logrus.Entry
var eventContext context.Context
var teamClient *team.Client

// eventWorker is a placeholder for the actual event worker function
var eventWorker = map[gensql.EventType]WorkerFunc{
	gensql.EventTypeCreateTeam: func(ctx context.Context, eventID uuid.UUID, anyForm any) {
		logger := newEventLogger(eventID)
		form, ok := anyForm.(team.Form)
		if !ok {
			logger.Fatalf("Illegal form type")
			return
		}
		go teamClient.Create(ctx, form, logger)
	},
}

func getTask(e *gensql.Event) (map[string]string, error) {
	var result map[string]string
	err := json.Unmarshal(e.Task, &result)
	return result, err
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

	var eventRetrievers = []func() ([]gensql.Event, error){
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
				} else {
					for _, event := range pickedEvents {
						if worker, ok := eventWorker[event.EventType]; ok {
							task, err := getTask(&event)
							if err != nil {
								err := querier.EventSetStatus(ctx, gensql.EventSetStatusParams{
									Status: gensql.EventStatusFailed,
								})
								if err != nil {
									log.Errorf("can't change status to %v for %v: %v\n", gensql.EventStatusFailed, event.EventType, err)
								}
								//or decent error handling like send slack bug notification?
								log.WithField("eventType", event.EventType).WithField("eventID", event.ID).Errorf("retrieved event with invalid param: %v", err)
							} else {
								worker(ctx, event.ID, task)
							}
						} else {
							log.Errorf("No worker found for event type %v\n", event.EventType)
						}
					}
				}
			}
		}
	}()
}
