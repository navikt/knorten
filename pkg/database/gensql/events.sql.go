// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.19.1
// source: events.sql

package gensql

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const eventCreate = `-- name: EventCreate :exec
INSERT INTO Events (owner, event_type, task, status, deadline)
VALUES ($1,
        $2,
        $3,
        'new',
        $4)
`

type EventCreateParams struct {
	Owner     string
	EventType EventType
	Task      json.RawMessage
	Deadline  time.Time
}

func (q *Queries) EventCreate(ctx context.Context, arg EventCreateParams) error {
	_, err := q.db.ExecContext(ctx, eventCreate,
		arg.Owner,
		arg.EventType,
		arg.Task,
		arg.Deadline,
	)
	return err
}

const eventLogCreate = `-- name: EventLogCreate :exec
INSERT INTO Event_Logs (event_id, log_type, message)
VALUES ($1, $2, $3)
`

type EventLogCreateParams struct {
	EventID uuid.UUID
	LogType LogType
	Message string
}

func (q *Queries) EventLogCreate(ctx context.Context, arg EventLogCreateParams) error {
	_, err := q.db.ExecContext(ctx, eventLogCreate, arg.EventID, arg.LogType, arg.Message)
	return err
}

const eventLogsForEventsGet = `-- name: EventLogsForEventsGet :many
SELECT events.event_type,
       events.status,
       events.deadline,
       events.created_at,
       events.updated_at,
       events.owner,
       json_agg(to_jsonb(el) - 'event_id')
FROM events
         JOIN (SELECT event_id, message, log_type, created_at FROM event_logs ORDER BY event_logs.created_at DESC) el
              ON el.event_id = events.id
GROUP BY events.id, events.updated_at
ORDER BY events.updated_at DESC
LIMIT $1
`

type EventLogsForEventsGetRow struct {
	EventType EventType
	Status    EventStatus
	Deadline  time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
	Owner     string
	JsonAgg   json.RawMessage
}

func (q *Queries) EventLogsForEventsGet(ctx context.Context, lim int32) ([]EventLogsForEventsGetRow, error) {
	rows, err := q.db.QueryContext(ctx, eventLogsForEventsGet, lim)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []EventLogsForEventsGetRow{}
	for rows.Next() {
		var i EventLogsForEventsGetRow
		if err := rows.Scan(
			&i.EventType,
			&i.Status,
			&i.Deadline,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.Owner,
			&i.JsonAgg,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const eventLogsForOwnerGet = `-- name: EventLogsForOwnerGet :many
SELECT events.event_type,
       events.status,
       events.deadline,
       events.created_at,
       events.updated_at,
       events.owner,
       json_agg(to_jsonb(el) - 'event_id')
FROM events
         JOIN (SELECT event_id, message, log_type, created_at FROM event_logs ORDER BY event_logs.created_at DESC) el
              ON el.event_id = events.id
WHERE owner = $1
GROUP BY events.id, events.updated_at
ORDER BY events.updated_at DESC
LIMIT $2
`

type EventLogsForOwnerGetParams struct {
	Owner string
	Lim   int32
}

type EventLogsForOwnerGetRow struct {
	EventType EventType
	Status    EventStatus
	Deadline  time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
	Owner     string
	JsonAgg   json.RawMessage
}

func (q *Queries) EventLogsForOwnerGet(ctx context.Context, arg EventLogsForOwnerGetParams) ([]EventLogsForOwnerGetRow, error) {
	rows, err := q.db.QueryContext(ctx, eventLogsForOwnerGet, arg.Owner, arg.Lim)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []EventLogsForOwnerGetRow{}
	for rows.Next() {
		var i EventLogsForOwnerGetRow
		if err := rows.Scan(
			&i.EventType,
			&i.Status,
			&i.Deadline,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.Owner,
			&i.JsonAgg,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const eventSetDeadline = `-- name: EventSetDeadline :exec
UPDATE
    Events
SET deadline = $1
WHERE id = $2
`

type EventSetDeadlineParams struct {
	Deadline time.Time
	ID       uuid.UUID
}

func (q *Queries) EventSetDeadline(ctx context.Context, arg EventSetDeadlineParams) error {
	_, err := q.db.ExecContext(ctx, eventSetDeadline, arg.Deadline, arg.ID)
	return err
}

const eventSetStatus = `-- name: EventSetStatus :exec
UPDATE
    Events
SET status = $1
WHERE id = $2
`

type EventSetStatusParams struct {
	Status EventStatus
	ID     uuid.UUID
}

func (q *Queries) EventSetStatus(ctx context.Context, arg EventSetStatusParams) error {
	_, err := q.db.ExecContext(ctx, eventSetStatus, arg.Status, arg.ID)
	return err
}

const eventsGetNew = `-- name: EventsGetNew :many
SELECT id, event_type, task, status, deadline, created_at, updated_at, owner
FROM Events
WHERE status = 'new'
ORDER BY created_at DESC
`

func (q *Queries) EventsGetNew(ctx context.Context) ([]Event, error) {
	rows, err := q.db.QueryContext(ctx, eventsGetNew)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Event{}
	for rows.Next() {
		var i Event
		if err := rows.Scan(
			&i.ID,
			&i.EventType,
			&i.Task,
			&i.Status,
			&i.Deadline,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.Owner,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const eventsGetOverdue = `-- name: EventsGetOverdue :many
SELECT id, event_type, task, status, deadline, created_at, updated_at, owner
FROM Events
WHERE status = 'new'
  AND deadline < NOW()
`

func (q *Queries) EventsGetOverdue(ctx context.Context) ([]Event, error) {
	rows, err := q.db.QueryContext(ctx, eventsGetOverdue)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Event{}
	for rows.Next() {
		var i Event
		if err := rows.Scan(
			&i.ID,
			&i.EventType,
			&i.Task,
			&i.Status,
			&i.Deadline,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.Owner,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const eventsGetPending = `-- name: EventsGetPending :many
SELECT id, event_type, task, status, deadline, created_at, updated_at, owner
FROM Events
WHERE status = 'pending'
ORDER BY created_at DESC
`

func (q *Queries) EventsGetPending(ctx context.Context) ([]Event, error) {
	rows, err := q.db.QueryContext(ctx, eventsGetPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Event{}
	for rows.Next() {
		var i Event
		if err := rows.Scan(
			&i.ID,
			&i.EventType,
			&i.Task,
			&i.Status,
			&i.Deadline,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.Owner,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
