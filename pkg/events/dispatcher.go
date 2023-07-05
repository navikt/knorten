package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/sirupsen/logrus"
)

type WorkerFunc func(uuid.UUID, map[string]string)

type EventType string

var started bool

var eventChan chan string

var log *logrus.Entry

// eventWorker is a placeholder for the actual event worker function
var eventWorker = map[EventType]WorkerFunc{
	"create:team": func(eventID uuid.UUID, param map[string]string) {
	},
	"update:team": func(eventID uuid.UUID, param map[string]string) {
	},
	"delete:team": func(eventID uuid.UUID, param map[string]string) {
	},
	"create:jupyter": func(eventID uuid.UUID, param map[string]string) {
	},
	"update:jupyter": func(eventID uuid.UUID, param map[string]string) {
	},
	"delete:jupyter": func(eventID uuid.UUID, param map[string]string) {
	},
	"create:airflow": func(eventID uuid.UUID, param map[string]string) {
	},
	"update:airflow": func(eventID uuid.UUID, param map[string]string) {
	},
	"delete:airflow": func(eventID uuid.UUID, param map[string]string) {
	},
}

func getEventType(e *gensql.Event) EventType {
	return EventType(string(e.Op) + ":" + string(e.ResourceType))
}

func getEventParam(e *gensql.Event) (map[string]string, error) {
	var result map[string]string
	err := json.Unmarshal([]byte(e.Param), &result)
	return result, err
}

func TriggerDispatcher(incomingEvent string) {
	if !started {
		return
	}
	eventChan <- incomingEvent
}

func Start(ctx context.Context, logEntry *logrus.Entry, querier gensql.Querier) {
	if started {
		log.Errorln("Event dispatcher already started!")
		return
	}

	started = true

	log = logEntry

	eventChan = make(chan string)

	var eventRetrievers []func() ([]gensql.Event, error) = []func() ([]gensql.Event, error){
		func() ([]gensql.Event, error) {
			return querier.EventsGetNew(ctx)
		},
		func() ([]gensql.Event, error) {
			return querier.EventsGetOverdue(ctx)
		},
	}

	go func() {
		for {
			for _, eventRetriever := range eventRetrievers {
				pickedEvents, err := eventRetriever()
				if err != nil {
					log.Errorf("Failed to fetch events: %v\n", err)
				} else {
					for _, event := range pickedEvents {
						if worker, ok := eventWorker[getEventType((&event))]; ok {
							eventParam, err := getEventParam(&event)
							if err != nil {
								querier.EventSetStatus(ctx, gensql.EventSetStatusParams{
									Status: "invalid",
								})
								//or decent error handling like send slack bug notification?
								log.Errorf("retrieved event with invalid param: %v", event)
							} else {
								worker(event.ID, eventParam)
							}
						} else {
							log.Errorf("No worker found for event type %v\n", getEventType(&event))
						}
					}
				}
			}

			select {
			case incomingEvent := <-eventChan:
				log.Debug("Received event: ", incomingEvent)
			case <-time.After(1 * time.Minute):
				log.Debug("Event dispatcher run!")
			}
		}
	}()
}
