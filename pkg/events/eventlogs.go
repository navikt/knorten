package events

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/sirupsen/logrus"
)

type EventLogger struct {
	eventID uuid.UUID
	log     *logrus.Entry
	repo    *database.Repo
	context context.Context
}

func (e EventLogger) Infof(template string, arg ...any) {
	message := fmt.Sprintf(template, arg...)
	e.log.Info(message)

	err := e.repo.EventLogCreate(e.context, e.eventID, message, gensql.LogTypeInfo)
	if err != nil {
		e.log.WithError(err).Error("can't write event to database")
	}
}

func (e EventLogger) Errorf(template string, arg ...any) {
	message := fmt.Sprintf(template, arg...)
	e.log.Error(message)

	err := e.repo.EventLogCreate(e.context, e.eventID, message, gensql.LogTypeError)
	if err != nil {
		e.log.WithError(err).Error("can't write event to database")
	}

	err = e.repo.EventSetDeadline(e.context, time.Now().Add(3*time.Minute))
	if err != nil {
		e.log.WithError(err).Errorf("can't extend the deadline for event")
	}

	err = e.repo.EventSetStatus(e.context, e.eventID, gensql.EventStatusPending)
	if err != nil {
		e.log.WithError(err).Errorf("can't set status to %v for event", gensql.EventStatusPending)
	}
}

func (e EventLogger) WithField(key string, value interface{}) *logrus.Entry {
	return e.log.WithFields(logrus.Fields{key: value})
}

func (e EventLogger) WithError(err error) *logrus.Entry {
	return e.log.WithField(logrus.ErrorKey, err)
}

func newEventLogger(ctx context.Context, log *logrus.Entry, repo *database.Repo, event gensql.Event) EventLogger {
	return EventLogger{
		eventID: event.ID,
		log:     log.WithField("eventType", event.EventType).WithField("eventID", event.ID),
		repo:    repo,
		context: ctx,
	}
}
