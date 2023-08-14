// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.20.0
// source: events.sql

package gensql

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const dispatcherEventsGet = `-- name: DispatcherEventsGet :many
SELECT id, event_type, payload, status, deadline, created_at, updated_at, owner, retry_count
FROM Events
WHERE status = 'new'
   OR (status = 'pending' AND updated_at + deadline::interval * retry_count < NOW())
ORDER BY created_at DESC
`

func (q *Queries) DispatcherEventsGet(ctx context.Context) ([]Event, error) {
	rows, err := q.db.QueryContext(ctx, dispatcherEventsGet)
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
			&i.Payload,
			&i.Status,
			&i.Deadline,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.Owner,
			&i.RetryCount,
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

const eventCreate = `-- name: EventCreate :exec
INSERT INTO Events (owner, event_type, payload, status, deadline)
VALUES ($1,
        $2,
        $3,
        'new',
        $4)
`

type EventCreateParams struct {
	Owner     string
	EventType EventType
	Payload   json.RawMessage
	Deadline  string
}

func (q *Queries) EventCreate(ctx context.Context, arg EventCreateParams) error {
	_, err := q.db.ExecContext(ctx, eventCreate,
		arg.Owner,
		arg.EventType,
		arg.Payload,
		arg.Deadline,
	)
	return err
}

const eventGet = `-- name: EventGet :one
SELECT id, event_type, payload, status, deadline, created_at, updated_at, owner, retry_count
FROM Events
WHERE id = $1
`

func (q *Queries) EventGet(ctx context.Context, id uuid.UUID) (Event, error) {
	row := q.db.QueryRowContext(ctx, eventGet, id)
	var i Event
	err := row.Scan(
		&i.ID,
		&i.EventType,
		&i.Payload,
		&i.Status,
		&i.Deadline,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Owner,
		&i.RetryCount,
	)
	return i, err
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

const eventLogsForEventGet = `-- name: EventLogsForEventGet :many
SELECT message, log_type, created_at::timestamptz
FROM event_logs
WHERE event_id = $1
ORDER BY created_at DESC
`

type EventLogsForEventGetRow struct {
	Message   string
	LogType   LogType
	CreatedAt time.Time
}

func (q *Queries) EventLogsForEventGet(ctx context.Context, id uuid.UUID) ([]EventLogsForEventGetRow, error) {
	rows, err := q.db.QueryContext(ctx, eventLogsForEventGet, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []EventLogsForEventGetRow{}
	for rows.Next() {
		var i EventLogsForEventGetRow
		if err := rows.Scan(&i.Message, &i.LogType, &i.CreatedAt); err != nil {
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
SELECT events.id, events.event_type, events.payload, events.status, events.deadline, events.created_at, events.updated_at, events.owner, events.retry_count,
       json_agg(el.*)        AS json_logs
FROM events
         JOIN (SELECT event_id, message, log_type, created_at::timestamptz
               FROM event_logs
               ORDER BY event_logs.created_at DESC
               LIMIT $1) el
              ON el.event_id = events.id
WHERE owner = $2
GROUP BY events.id, events.updated_at
ORDER BY events.updated_at DESC
LIMIT $1
`

type EventLogsForOwnerGetParams struct {
	Lim   int32
	Owner string
}

type EventLogsForOwnerGetRow struct {
	ID         uuid.UUID
	EventType  EventType
	Payload    json.RawMessage
	Status     EventStatus
	Deadline   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Owner      string
	RetryCount int32
	JsonLogs   json.RawMessage
}

func (q *Queries) EventLogsForOwnerGet(ctx context.Context, arg EventLogsForOwnerGetParams) ([]EventLogsForOwnerGetRow, error) {
	rows, err := q.db.QueryContext(ctx, eventLogsForOwnerGet, arg.Lim, arg.Owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []EventLogsForOwnerGetRow{}
	for rows.Next() {
		var i EventLogsForOwnerGetRow
		if err := rows.Scan(
			&i.ID,
			&i.EventType,
			&i.Payload,
			&i.Status,
			&i.Deadline,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.Owner,
			&i.RetryCount,
			&i.JsonLogs,
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

const eventSetPendingStatus = `-- name: EventSetPendingStatus :exec
UPDATE Events
SET status      = 'pending',
    retry_count = retry_count + 1
WHERE id = $1
`

func (q *Queries) EventSetPendingStatus(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, eventSetPendingStatus, id)
	return err
}

const eventSetStatus = `-- name: EventSetStatus :exec
UPDATE Events
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

const eventsByOwnerGet = `-- name: EventsByOwnerGet :many
SELECT id, event_type, payload, status, deadline, created_at, updated_at, owner, retry_count
FROM Events
WHERE owner = $1
ORDER BY updated_at DESC
LIMIT $2
`

type EventsByOwnerGetParams struct {
	Owner string
	Lim   int32
}

func (q *Queries) EventsByOwnerGet(ctx context.Context, arg EventsByOwnerGetParams) ([]Event, error) {
	rows, err := q.db.QueryContext(ctx, eventsByOwnerGet, arg.Owner, arg.Lim)
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
			&i.Payload,
			&i.Status,
			&i.Deadline,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.Owner,
			&i.RetryCount,
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

const eventsGetType = `-- name: EventsGetType :many
SELECT id, event_type, payload, status, deadline, created_at, updated_at, owner, retry_count
FROM Events
WHERE event_type = $1
`

func (q *Queries) EventsGetType(ctx context.Context, eventType EventType) ([]Event, error) {
	rows, err := q.db.QueryContext(ctx, eventsGetType, eventType)
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
			&i.Payload,
			&i.Status,
			&i.Deadline,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.Owner,
			&i.RetryCount,
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
