-- name: EventCreate :exec
INSERT INTO Events (owner, event_type, task, status, deadline)
VALUES (@owner,
        @event_type,
        @task,
        'new',
        @deadline);

-- name: EventsGetNew :many
SELECT *
FROM Events
WHERE status = 'new'
ORDER BY created_at DESC;

-- name: EventsGetPending :many
SELECT *
FROM Events
WHERE status = 'pending'
ORDER BY created_at DESC;

-- name: EventsGetOverdue :many
SELECT *
FROM Events
WHERE status = 'new'
  AND deadline < NOW();

-- name: EventSetDeadline :exec
UPDATE
    Events
SET deadline = @deadline
WHERE id = @id;

-- name: EventSetStatus :exec
UPDATE
    Events
SET status = @status
WHERE id = @id;

-- name: EventLogCreate :exec
INSERT INTO Event_Logs (event_id, log_type, message)
VALUES (@event_id, @log_type, @message);

-- name: EventLogsForEventsGet :many
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
LIMIT @lim;

-- name: EventLogsForOwnerGet :many
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
WHERE owner = @owner
GROUP BY events.id, events.updated_at
ORDER BY events.updated_at DESC
LIMIT @lim;
