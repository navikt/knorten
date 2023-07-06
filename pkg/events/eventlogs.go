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

func (el EventLogger) Infof(messageTemplate string, arg ...any) {
	message := fmt.Sprintf(messageTemplate, arg)
	log.Info(message)
	dbQuerier.EventLogCreate(eventContext, gensql.EventLogCreateParams{
		EventID: el.EventID,
		Message: message,
		LogType: gensql.LogTypeInfo,
	})
}

func (el EventLogger) Errorf(messageTemplate string, arg ...any) {
	message := fmt.Sprintf(messageTemplate, arg)
	log.Error(message)
	dbQuerier.EventLogCreate(eventContext, gensql.EventLogCreateParams{
		EventID: el.EventID,
		Message: message,
		LogType: gensql.LogTypeError,
	})
	dbQuerier.EventSetDeadline(eventContext, gensql.EventSetDeadlineParams{
		Deadline: time.Now().Add(3 * time.Minute),
	})
	dbQuerier.EventSetStatus(eventContext, gensql.EventSetStatusParams{
		Status: gensql.EventStatusPending,
	})
}

func (el EventLogger) Fatalf(messageTemplate string, arg ...any) {
	message := fmt.Sprintf(messageTemplate, arg)
	log.Fatal(message)
	dbQuerier.EventLogCreate(eventContext, gensql.EventLogCreateParams{
		EventID: el.EventID,
		Message: message,
		LogType: gensql.LogTypeFatal,
	})
	dbQuerier.EventSetStatus(eventContext, gensql.EventSetStatusParams{
		Status: gensql.EventStatusFailed,
	})
}

func (el EventLogger) Debugf(messageTemplate string, arg ...any) {
	message := fmt.Sprintf(messageTemplate, arg)
	log.Info(message)
}

func newEventLogger(eventID uuid.UUID) EventLogger {
	return EventLogger{
		EventID: eventID,
	}
}
