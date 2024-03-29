// Code generated by sqlc. DO NOT EDIT.
// source: events.sql

package gensql

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
)

const eventCreate = `-- name: EventCreate :exec
INSERT INTO Events (owner, type, payload, status, deadline)
VALUES ($1,
        $2,
        $3,
        'new',
        $4)
`

type EventCreateParams struct {
	Owner    string
	Type     string
	Payload  json.RawMessage
	Deadline string
}

func (q *Queries) EventCreate(ctx context.Context, arg EventCreateParams) error {
	_, err := q.db.ExecContext(ctx, eventCreate,
		arg.Owner,
		arg.Type,
		arg.Payload,
		arg.Deadline,
	)
	return err
}

const eventGet = `-- name: EventGet :one
SELECT id, type, payload, status, deadline, created_at, updated_at, owner, retry_count
FROM Events
WHERE id = $1
`

func (q *Queries) EventGet(ctx context.Context, id uuid.UUID) (Event, error) {
	row := q.db.QueryRowContext(ctx, eventGet, id)
	var i Event
	err := row.Scan(
		&i.ID,
		&i.Type,
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

const eventIncrementRetryCount = `-- name: EventIncrementRetryCount :exec
UPDATE events
SET retry_count = retry_count + 1
WHERE id = $1
`

func (q *Queries) EventIncrementRetryCount(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, eventIncrementRetryCount, id)
	return err
}

const eventLogCreate = `-- name: EventLogCreate :exec
INSERT INTO Event_Logs (event_id, log_type, message)
VALUES ($1, $2, $3)
`

type EventLogCreateParams struct {
	EventID uuid.UUID
	LogType string
	Message string
}

func (q *Queries) EventLogCreate(ctx context.Context, arg EventLogCreateParams) error {
	_, err := q.db.ExecContext(ctx, eventLogCreate, arg.EventID, arg.LogType, arg.Message)
	return err
}

const eventLogsForEventGet = `-- name: EventLogsForEventGet :many
SELECT id, event_id, log_type, message, created_at
FROM event_logs
WHERE event_id = $1
ORDER BY created_at DESC
`

func (q *Queries) EventLogsForEventGet(ctx context.Context, id uuid.UUID) ([]EventLog, error) {
	rows, err := q.db.QueryContext(ctx, eventLogsForEventGet, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []EventLog{}
	for rows.Next() {
		var i EventLog
		if err := rows.Scan(
			&i.ID,
			&i.EventID,
			&i.LogType,
			&i.Message,
			&i.CreatedAt,
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

const eventSetStatus = `-- name: EventSetStatus :exec
UPDATE Events
SET status = $1
WHERE id = $2
`

type EventSetStatusParams struct {
	Status string
	ID     uuid.UUID
}

func (q *Queries) EventSetStatus(ctx context.Context, arg EventSetStatusParams) error {
	_, err := q.db.ExecContext(ctx, eventSetStatus, arg.Status, arg.ID)
	return err
}

const eventsByOwnerGet = `-- name: EventsByOwnerGet :many
SELECT id, type, payload, status, deadline, created_at, updated_at, owner, retry_count
FROM Events
WHERE owner = $1
ORDER BY updated_at DESC
LIMIT $2
`

type EventsByOwnerGetParams struct {
	Owner string
	Lim   sql.NullInt32
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
			&i.Type,
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
SELECT id, type, payload, status, deadline, created_at, updated_at, owner, retry_count
FROM Events
WHERE type = $1
`

func (q *Queries) EventsGetType(ctx context.Context, eventType string) ([]Event, error) {
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
			&i.Type,
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

const eventsProcessingGet = `-- name: EventsProcessingGet :many
SELECT id, type, payload, status, deadline, created_at, updated_at, owner, retry_count
FROM events
WHERE status = 'processing'
ORDER BY created_at DESC
`

func (q *Queries) EventsProcessingGet(ctx context.Context) ([]Event, error) {
	rows, err := q.db.QueryContext(ctx, eventsProcessingGet)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Event{}
	for rows.Next() {
		var i Event
		if err := rows.Scan(
			&i.ID,
			&i.Type,
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

const eventsReset = `-- name: EventsReset :exec
UPDATE events
SET status = 'pending'
WHERE status = 'processing'
`

func (q *Queries) EventsReset(ctx context.Context) error {
	_, err := q.db.ExecContext(ctx, eventsReset)
	return err
}

const eventsUpcomingGet = `-- name: EventsUpcomingGet :many
SELECT id, type, payload, status, deadline, created_at, updated_at, owner, retry_count
FROM Events
WHERE status = 'new'
   OR status = 'pending'
   OR status = 'deadline_reached'
ORDER BY created_at ASC
`

func (q *Queries) EventsUpcomingGet(ctx context.Context) ([]Event, error) {
	rows, err := q.db.QueryContext(ctx, eventsUpcomingGet)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Event{}
	for rows.Next() {
		var i Event
		if err := rows.Scan(
			&i.ID,
			&i.Type,
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
