-- name: EventCreate :exec
INSERT INTO
    Events (op, resource_type, param, status, deadline)
VALUES
    (
        @op,
        @resource_type,
        @param,
        'new',
        NOW() + INTERVAL @duration
    );

-- name: EventGet :one
SELECT
    id,
    op,
    resource_type,
    param,
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
    status = "new"
ORDER BY
    created_at DESC;

-- name: EventsGetOverdue :many
SELECT
    *
FROM
    Events
WHERE
    status = "new"
    AND deadline < NOW();

-- name: EventProlongDeadline :exec
UPDATE
    Events
SET
    deadline = deadline + INTERVAL @duration
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
    Event_Logs (event_id, message)
VALUES
    (@event_id, @message);

-- name: EventLogsForEventGet :many
SELECT
    id,
    event_id,
    message,
    created_at
FROM
    Event_Logs
WHERE
    event_id = @event_id
ORDER BY
    created_at DESC;