package events

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/sirupsen/logrus"
)

type EventLogger struct {
	eventID uuid.UUID
	log     *logrus.Entry
}

func (el EventLogger) Infof(template string, arg ...any) {
	message := fmt.Sprintf(template, arg...)
	el.log.Info(message)

	err := dbQuerier.EventLogCreate(eventContext, gensql.EventLogCreateParams{
		EventID: el.eventID,
		Message: message,
		LogType: gensql.LogTypeInfo,
	})
	if err != nil {
		el.log.WithError(err).Error("can't write event to database")
	}
}

func (el EventLogger) Errorf(template string, arg ...any) {
	message := fmt.Sprintf(template, arg...)
	el.log.Error(message)

	err := dbQuerier.EventLogCreate(eventContext, gensql.EventLogCreateParams{
		EventID: el.eventID,
		Message: message,
		LogType: gensql.LogTypeError,
	})
	if err != nil {
		el.log.WithError(err).Error("can't write event to database")
	}

	err = dbQuerier.EventSetDeadline(eventContext, gensql.EventSetDeadlineParams{
		Deadline: time.Now().Add(3 * time.Minute),
	})
	if err != nil {
		el.log.WithError(err).Errorf("can't extend the deadline for event")
	}

	err = dbQuerier.EventSetStatus(eventContext, gensql.EventSetStatusParams{
		Status: gensql.EventStatusPending,
	})
	if err != nil {
		el.log.WithError(err).Errorf("can't set status to %v for event", gensql.EventStatusPending)
	}
}

func newEventLogger(event gensql.Event) EventLogger {
	return EventLogger{
		eventID: event.ID,
		log:     log.WithField("eventType", event.EventType).WithField("eventID", event.ID),
	}
}
