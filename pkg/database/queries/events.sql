-- name: EventCreate :exec
INSERT INTO Events (owner, event_type, payload, status, deadline)
VALUES (@owner,
        @event_type,
        @payload,
        'new',
        @deadline);

-- name: EventGet :one
SELECT events.id,
       events.event_type,
       events.status,
       events.deadline::TEXT as deadline,
       events.created_at,
       events.updated_at,
       events.owner,
       events.retry_count,
       events.payload
FROM Events
WHERE id = @id;

-- name: EventsByOwnerGet :many
SELECT events.id,
       events.event_type,
       events.status,
       events.deadline::TEXT as deadline,
       events.created_at,
       events.updated_at,
       events.owner,
       events.retry_count,
       events.payload
FROM Events
WHERE owner = @owner
ORDER BY updated_at DESC
LIMIT @lim;

-- name: EventsGetNew :many
SELECT id, owner, event_type, payload
FROM Events
WHERE status = 'new'
ORDER BY created_at DESC;

-- name: EventsGetOverdue :many
SELECT id, owner, event_type, payload
FROM Events
WHERE status = 'pending'
  AND updated_at + deadline * retry_count < NOW();

-- name: EventSetStatus :exec
UPDATE
    Events
SET status = @status
WHERE id = @id;

-- name: EventSetPendingStatus :exec
UPDATE
    Events
SET status      = 'pending',
    retry_count = retry_count + 1
WHERE id = @id;

-- name: EventLogCreate :exec
INSERT INTO Event_Logs (event_id, log_type, message)
VALUES (@event_id, @log_type, @message);

-- name: EventLogsForEventGet :many
SELECT message, log_type, created_at::timestamptz
FROM event_logs
WHERE event_id = @id
ORDER BY created_at DESC;

-- name: EventLogsForOwnerGet :many
SELECT events.event_type,
       events.status,
       events.deadline::TEXT as deadline,
       events.created_at,
       events.updated_at,
       events.owner,
       events.retry_count,
       json_agg(el.*)        AS json_logs
FROM events
         JOIN (SELECT event_id, message, log_type, created_at::timestamptz
               FROM event_logs
               ORDER BY event_logs.created_at DESC
               LIMIT @lim) el
              ON el.event_id = events.id
WHERE owner = @owner
GROUP BY events.id, events.updated_at
ORDER BY events.updated_at DESC
LIMIT @lim;
