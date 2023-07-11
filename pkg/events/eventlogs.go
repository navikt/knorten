package events

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/database/gensql"
)

type EventLogger struct {
	EventID uuid.UUID
}

func (el EventLogger) Infof(template string, arg ...any) {
	message := fmt.Sprintf(template, arg...)
	log.Info(message)
	err := dbQuerier.EventLogCreate(eventContext, gensql.EventLogCreateParams{
		EventID: el.EventID,
		Message: message,
		LogType: gensql.LogTypeInfo,
	})
	if err != nil {
		log.Errorf("can't write event(%v) to database: %v", el.EventID, err)
	}
}

func (el EventLogger) Errorf(messageTemplate string, arg ...any) {
	message := fmt.Sprintf(messageTemplate, arg...)
	log.Error(message)
	err := dbQuerier.EventLogCreate(eventContext, gensql.EventLogCreateParams{
		EventID: el.EventID,
		Message: message,
		LogType: gensql.LogTypeError,
	})
	if err != nil {
		log.Errorf("can't write event(%v) to database: %v", el.EventID, err)
	}
	err = dbQuerier.EventSetDeadline(eventContext, gensql.EventSetDeadlineParams{
		Deadline: time.Now().Add(3 * time.Minute),
	})
	if err != nil {
		log.Errorf("can't extend the deadline for event(%v) in database: %v", el.EventID, err)
	}
	err = dbQuerier.EventSetStatus(eventContext, gensql.EventSetStatusParams{
		Status: gensql.EventStatusPending,
	})
	if err != nil {
		log.Errorf("can't set status to %v for event(%v) in database: %v", gensql.EventStatusPending, el.EventID, err)
	}
}

func (el EventLogger) Fatalf(messageTemplate string, arg ...any) {
	message := fmt.Sprintf(messageTemplate, arg...)
	err := dbQuerier.EventLogCreate(eventContext, gensql.EventLogCreateParams{
		EventID: el.EventID,
		Message: message,
		LogType: gensql.LogTypeFatal,
	})
	if err != nil {
		log.Errorf("can't write event(%v) to database: %v", el.EventID, err)
	}
	err = dbQuerier.EventSetStatus(eventContext, gensql.EventSetStatusParams{
		Status: gensql.EventStatusFailed,
	})
	if err != nil {
		log.Errorf("can't set status to %v for event(%v) in database: %v", gensql.EventStatusFailed, el.EventID, err)
	}
	log.Fatal(message)
}

func (el EventLogger) Debugf(messageTemplate string, arg ...any) {
	message := fmt.Sprintf(messageTemplate, arg...)
	log.Info(message)
}

func newEventLogger(eventID uuid.UUID) EventLogger {
	return EventLogger{
		EventID: eventID,
	}
}
