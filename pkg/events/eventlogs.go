package events

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/sirupsen/logrus"
)

type EventLogger struct {
	eventID uuid.UUID
	log     *logrus.Entry
	repo    database.Repository
	context context.Context
}

func (e EventLogger) Info(messages ...any) {
	for _, message := range messages {
		messageAsString := fmt.Sprint(message)

		e.log.Info(messageAsString)
		err := e.repo.EventLogCreate(e.context, e.eventID, messageAsString, database.LogTypeInfo)
		if err != nil {
			e.log.WithError(err).Error("can't write event to database")
		}
	}
}

func (e EventLogger) Infof(template string, arg ...any) {
	e.Info(fmt.Sprintf(template, arg...))
}

// Error will not create event logs for users.
func (e EventLogger) Error(messages ...any) {
	for _, message := range messages {
		messageAsString := fmt.Sprint(message)
		e.log.Error(messageAsString)
	}
}

// Errorf will not create event logs for users.
func (e EventLogger) Errorf(template string, arg ...any) {
	e.Error(fmt.Sprintf(template, arg...))
}

func (e EventLogger) WithError(err error) *logrus.Entry {
	return e.log.WithError(err)
}

func newEventLogger(ctx context.Context, log *logrus.Entry, repo database.Repository, event gensql.Event) EventLogger {
	return EventLogger{
		eventID: event.ID,
		log:     log.WithField("eventType", event.Type).WithField("eventID", event.ID),
		repo:    repo,
		context: ctx,
	}
}
