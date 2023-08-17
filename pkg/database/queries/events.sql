-- name: EventCreate :exec
INSERT INTO Events (owner, type, payload, status, deadline)
VALUES (@owner,
        @type,
        @payload,
        'new',
        @deadline);

-- name: EventGet :one
SELECT *
FROM Events
WHERE id = @id;

-- name: EventsByOwnerGet :many
SELECT *
FROM Events
WHERE owner = @owner
ORDER BY updated_at DESC
LIMIT @lim;

-- name: DispatcherEventsProcessingGet :many
SELECT *
FROM events
WHERE status = 'processing'
ORDER BY created_at DESC;

-- name: DispatcherEventsUpcomingGet :many
SELECT *
FROM Events
WHERE status = 'new'
   OR (status = 'pending' AND updated_at + deadline::interval * retry_count < NOW())
ORDER BY created_at ASC;

-- name: EventsGetType :many
SELECT *
FROM Events
WHERE type = @event_type;

-- name: EventSetStatus :exec
UPDATE Events
SET status = @status
WHERE id = @id;

-- name: EventSetPendingStatus :exec
UPDATE Events
SET status      = 'pending',
    retry_count = retry_count + 1
WHERE id = @id;

-- name: EventLogCreate :exec
INSERT INTO Event_Logs (event_id, log_type, message)
VALUES (@event_id, @log_type, @message);

-- name: EventLogsForEventGet :many
SELECT *
FROM event_logs
WHERE event_id = @id
ORDER BY created_at DESC;
