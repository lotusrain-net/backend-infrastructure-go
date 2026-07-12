# SP-06 Generic Task Infrastructure

## Ownership

`internal/modules/task/`, `internal/platform/queue/`, `internal/platform/scheduler/`, task wiring in `cmd/worker` and `cmd/scheduler`.

## Deliverables

- Task definition, execution, and schedule domain/application layers.
- String-based handler registry with no business task enum.
- Asynq publisher, worker, retry policy, unique-task idempotency, delayed tasks, periodic scheduling, and failure archive.
- State flow: queued, running, succeeded, failed, cancelled.
- System test task used only for end-to-end verification.

## Tests

- Registry behavior, invalid transitions, duplicate submission, retry exhaustion, schedule refresh, worker interruption, failure archive, and PostgreSQL status synchronization.

## Exclusions

- No collection, ETL, browser, analytics, or Douyin task.
