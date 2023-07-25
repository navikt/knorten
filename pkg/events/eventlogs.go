package events

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
	"github.com/sirupsen/logrus"
)

type EventLogger struct {
	eventID uuid.UUID
	log     *logrus.Entry
	repo    *database.Repo
	context context.Context
}

func (e EventLogger) Info(messages ...any) {
	for _, message := range messages {
		messageAsString := fmt.Sprint(message)

		e.log.Info(messageAsString)
		err := e.repo.EventLogCreate(e.context, e.eventID, messageAsString, gensql.LogTypeInfo)
		if err != nil {
			e.log.WithError(err).Error("can't write event to database")
		}
	}
}

func (e EventLogger) Infof(template string, arg ...any) {
	e.Info(fmt.Sprintf(template, arg...))
}

func (e EventLogger) Error(messages ...any) {
	for _, message := range messages {
		messageAsString := fmt.Sprint(message)

		e.log.Error(messageAsString)
		err := e.repo.EventLogCreate(e.context, e.eventID, messageAsString, gensql.LogTypeError)
		if err != nil {
			e.log.WithError(err).Error("can't write event to database")
		}
	}

	err := e.repo.EventSetDeadline(e.context, time.Now().Add(3*time.Minute))
	if err != nil {
		e.log.WithError(err).Errorf("can't extend the deadline for event")
	}
}

func (e EventLogger) Errorf(template string, arg ...any) {
	e.Error(fmt.Sprintf(template, arg...))
}

func (e EventLogger) WithField(key string, value interface{}) logger.Logger {
	e.log = e.log.WithFields(logrus.Fields{key: value})
	return e
}

func (e EventLogger) WithError(err error) logger.Logger {
	e.log = e.log.WithField(logrus.ErrorKey, err)
	return e
}

func newEventLogger(ctx context.Context, log *logrus.Entry, repo *database.Repo, event gensql.Event) EventLogger {
	return EventLogger{
		eventID: event.ID,
		log:     log.WithField("eventType", event.EventType).WithField("eventID", event.ID),
		repo:    repo,
		context: ctx,
	}
}
