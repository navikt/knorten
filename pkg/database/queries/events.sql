-- name: EventCreate :exec
INSERT INTO
    Events (event_type, task, status, deadline)
VALUES
    (
        @event_type,
        @task,
        'new',
        @deadline
    );

-- name: EventGet :one
SELECT
    id,
    event_type,
    task,
    status,
    deadline,
    created_at,
    updated_at
FROM
    Events
WHERE
    id = @id;

-- name: EventsGetNew :many
SELECT
    *
FROM
    Events
WHERE
    status = 'new'
ORDER BY
    created_at DESC;

-- name: EventsGetPending :many
SELECT
    *
FROM
    Events
WHERE
        status = 'pending'
ORDER BY
    created_at DESC;

-- name: EventsGetOverdue :many
SELECT
    *
FROM
    Events
WHERE
    status = 'new'
    AND deadline < NOW();

-- name: EventSetDeadline :exec
UPDATE
    Events
SET
    deadline = @deadline
WHERE
    id = @id;

-- name: EventSetStatus :exec
UPDATE
    Events
SET
    status = @status
WHERE
    id = @id;

-- name: EventLogCreate :exec
INSERT INTO
    Event_Logs (event_id, log_type, message)
VALUES
    (@event_id, @log_type, @message);

-- name: EventLogsForEventGet :many
SELECT
    id,
    event_id,
    log_type,
    message,
    created_at
FROM
    Event_Logs
WHERE
    event_id = @event_id
ORDER BY
    created_at DESC;
