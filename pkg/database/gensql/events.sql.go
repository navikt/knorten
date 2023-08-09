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
	Deadline  int64
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
SELECT events.id,
       events.event_type,
       events.status,
       events.deadline::TEXT as deadline,
       events.created_at,
       events.updated_at,
       events.owner,
       events.retry_count,
       events.payload,
       json_agg(el.*) AS json_logs
FROM events
         JOIN (SELECT event_id, message, log_type, created_at::timestamptz
               FROM event_logs
               ORDER BY event_logs.created_at DESC
               LIMIT $1) el
              ON el.event_id = events.id
GROUP BY events.id, events.updated_at
ORDER BY events.updated_at DESC
LIMIT $1
`

type EventLogsForEventsGetRow struct {
	ID         uuid.UUID
	EventType  EventType
	Status     EventStatus
	Deadline   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Owner      string
	RetryCount int32
	Payload    json.RawMessage
	JsonLogs   json.RawMessage
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
			&i.ID,
			&i.EventType,
			&i.Status,
			&i.Deadline,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.Owner,
			&i.RetryCount,
			&i.Payload,
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

const eventLogsForOwnerGet = `-- name: EventLogsForOwnerGet :many
SELECT events.event_type,
       events.status,
       events.deadline::TEXT as deadline,
       events.created_at,
       events.updated_at,
       events.owner,
       events.retry_count,
       json_agg(el.*) AS json_logs
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
	EventType  EventType
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
			&i.EventType,
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
UPDATE
    Events
SET status = 'pending',
    retry_count = retry_count + 1
WHERE id = $1
`

func (q *Queries) EventSetPendingStatus(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, eventSetPendingStatus, id)
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
SELECT id, owner, event_type, payload
FROM Events
WHERE status = 'new'
ORDER BY created_at DESC
`

type EventsGetNewRow struct {
	ID        uuid.UUID
	Owner     string
	EventType EventType
	Payload   json.RawMessage
}

func (q *Queries) EventsGetNew(ctx context.Context) ([]EventsGetNewRow, error) {
	rows, err := q.db.QueryContext(ctx, eventsGetNew)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []EventsGetNewRow{}
	for rows.Next() {
		var i EventsGetNewRow
		if err := rows.Scan(
			&i.ID,
			&i.Owner,
			&i.EventType,
			&i.Payload,
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
SELECT id, owner, event_type, payload
FROM Events
WHERE status = 'pending'
  AND updated_at + deadline * retry_count < NOW()
`

type EventsGetOverdueRow struct {
	ID        uuid.UUID
	Owner     string
	EventType EventType
	Payload   json.RawMessage
}

func (q *Queries) EventsGetOverdue(ctx context.Context) ([]EventsGetOverdueRow, error) {
	rows, err := q.db.QueryContext(ctx, eventsGetOverdue)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []EventsGetOverdueRow{}
	for rows.Next() {
		var i EventsGetOverdueRow
		if err := rows.Scan(
			&i.ID,
			&i.Owner,
			&i.EventType,
			&i.Payload,
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
