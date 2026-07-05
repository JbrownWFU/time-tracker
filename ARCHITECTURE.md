# Architecture

## Overview

`time-tracker` is a Go CLI for tracking time spent on jobs. The domain model
is built around two core structs, `Job` and `Span`, defined in
[src/main.go](src/main.go). Persistence is backed by SQLite via
[src/sqldb.go](src/sqldb.go).

## Core structs

### `Job`

```go
type Job struct {
    ID          int
    Name        string
    Description string
    Status      string
}
```

A `Job` is the unit of work being tracked (e.g. a project or task). `Status`
tracks its lifecycle (e.g. `"todo"`).

Maps to the `Jobs` table:

```sql
create table if not exists Jobs (
    id int primary key,
    name text not null unique,
    desc text,
    created_at text default current_timestamp,
    status text not null
)
```

Note: `name` is unique at the storage layer but this is not yet reflected
as a constraint on the `Job` struct itself.

### `Span`

```go
type Span struct {
    ID        int
    JobID     int
    StartTime string
    EndTime   string
    Note      string
}
```

A `Span` is a single logged time span against a `Job`, linked via
`JobID` (foreign key relationship, not yet enforced in schema). `StartTime`
and `EndTime` are currently plain strings rather than a time type.

There is no `Spans` table yet — only `Jobs` is created by `MakeTables`.

## Relationship

```
Job (1) ──< (many) Span
  Job.ID  <──  Span.JobID
```

A `Job` owns zero or more `Span` records, each representing a discrete
period of work logged against it.

## Persistence layer

`SqlConn` ([src/sqldb.go](src/sqldb.go)) wraps a `*sql.DB` (SQLite, via
`modernc.org/sqlite`, a pure-Go driver requiring no CGo):

- `NewSqlConn(path string)` — opens/creates the DB file and pings it.
- `Close()` — closes the connection.
- `MakeTables()` — creates the `Jobs` table if it doesn't exist.
- `WriteJob(name, desc string)` — intended to persist a new `Job`; not yet
  implemented.

## Current state / gaps

This is an early-stage skeleton:

- No `Spans` table or `WriteSpan`/read methods yet.
- `WriteJob` is unimplemented (no body).
- No read/query methods (list jobs, list spans for a job, etc.).
- `main.go` currently only demonstrates constructing a `Job` and connecting
  to the DB — no real CLI flow yet.
