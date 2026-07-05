# Architecture

## Overview

`time-tracker` is a Go CLI for tracking time spent on jobs. The domain model
is built around three structs, `Job`, `Span`, and `Note`, defined in
[src/structs.go](src/structs.go). Persistence is backed by SQLite via
[src/sqldb.go](src/sqldb.go).

## Core structs

### `Job`

```go
type Job struct {
    ID     int
    Name   string
    Desc   string
    Status string
}
```

A `Job` is the unit of work being tracked (e.g. a project or task). `Status`
tracks its lifecycle and is validated against `validStatuses` (`"todo"`,
`"active"`, `"done"`) at the persistence layer.

Maps to the `Jobs` table:

```sql
create table if not exists Jobs (
    id integer primary key,
    name text not null unique,
    desc text not null,
    created_at text not null,
    status text not null
)
```

### `Span`

```go
type Span struct {
    ID        int
    JobID     int
    StartTime string
    EndTime   *string
}
```

A `Span` is a single logged clock-in/clock-out period against a `Job`,
linked via `JobID` (foreign key to `Jobs.id`). `EndTime` is a pointer so an
open (in-progress) span can be represented as `NULL` until clocked out.
Timestamps are stored as UTC text in `sqlTimeFormat`
(`"2006-01-02 15:04:05"`).

Maps to the `Spans` table:

```sql
create table if not exists Spans (
    id integer primary key,
    job_id integer references Jobs(id),
    start_time text not null,
    end_time text null
)
```

### `Note`

```go
type Note struct {
    ID      int
    EntryID int
    Content string
}
```

A `Note` is a free-text annotation attached to a `Span` (`EntryID` refers to
`Spans.id`). This was split out from `Span` so a span can carry structured
notes independent of the time-tracking fields. The table exists but there
are no `WriteNote`/read methods yet — notes are not wired up end-to-end.

Maps to the `Notes` table:

```sql
create table if not exists Notes (
    id integer primary key,
    entry_id integer references Spans(id),
    content text
)
```

## Relationship

```
Job (1) ──< (many) Span (1) ──< (many) Note
  Job.ID  <──  Span.JobID       Span.ID <── Note.EntryID
```

A `Job` owns zero or more `Span` records, each representing a discrete
period of work logged against it. Each `Span` may in turn own zero or more
`Note` records (not yet reachable from code).

## Persistence layer

`SqlConn` ([src/sqldb.go](src/sqldb.go)) wraps a `*sql.DB` (SQLite, via
`modernc.org/sqlite`, a pure-Go driver requiring no CGo):

- `NewSqlConn(path string)` — opens/creates the DB file and pings it.
- `Close()` — closes the connection.
- `MakeTables()` — creates `Jobs`, `Spans`, and `Notes` if they don't exist.

Job management:
- `WriteJob(name, desc, status string) (int, error)` — inserts a `Job`
  (validates `status`) and returns its ID.
- `GetJob(id int) (Job, error)` — fetches a `Job` by ID.
- `ResolveJob(name string) (int, error)` — looks up a `Job` ID by name.
- `UpdateJobStatus(id int, status string) (int, error)` — validates and
  updates a `Job`'s status.

Time spans:
- `WriteSpan(jobId int, startTime time.Time) (int, error)` — opens a new
  span (`end_time` left `NULL`) and returns its ID. Accepting `startTime`
  rather than always using `time.Now()` lets callers backdate a clock-in.
- `UpdateSpan(spanId int, endTime time.Time) error` — closes a span by
  setting `end_time`; accepting `endTime` similarly supports backdated
  clock-outs.

## Current state / gaps

- `Notes` table exists and the `Note` struct is defined, but there are no
  `WriteNote`/`GetNote` methods — notes aren't usable yet (tracked as a
  TODO in [src/main.go](src/main.go)).
- The `getSpan` and `getOpenSpanID` queries are defined in
  [src/sqldb.go](src/sqldb.go) but have no corresponding Go methods yet —
  there's currently no way to read a span back or detect an already-open
  span for a job. In particular, nothing stops `WriteSpan` from opening a
  second span for a job that already has one open (double clock-in).
- `main.go` is still a hardcoded demo (create a job, clock in, sleep, clock
  out) rather than a real CLI — there's no argument parsing or command
  dispatch, and `ResolveJob` (name → ID) isn't called from the demo flow
  even though it's implemented.
- Status validation (`validStatuses` membership check) lives in the
  persistence layer; a comment in `sqldb.go` notes this business logic
  should move to an API layer above `SqlConn`.
