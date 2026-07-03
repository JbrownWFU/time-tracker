# Structuring core + interfaces + implementations

## The confusion this resolves
It's tempting to think the `Repository` interface belongs bundled with its
implementations (`TestRepo`, `SQLiteRepo`) since that's "where the interface
gets used." It's actually the opposite: the interface belongs with the
**consumer** (`Service`), not the implementer.

## Why
`Service` is defined entirely in terms of the `Repository` interface - it
doesn't know or care what satisfies it. That's the whole payoff of Go's
structural typing: an implementation just needs a matching method set, with
no import of the core package required. So the interface's natural home is
next to the thing that depends on it, not next to the thing that fulfills it.

## Resulting shape

**Core** (one file, or one package): everything `Service` needs to be
understood on its own.
- Domain structs: `Job`, `Entry`
- The `Repository` interface (the contract)
- `Service` (business logic, depends only on the interface)

**Implementations** (separate files, or separate packages later):
- `TestRepo` - in-memory, for fast iteration / tests
- `SQLiteRepo` - real backing store (`modernc.org/sqlite`)

These only need the `Job`/`Entry` types from core. They don't need to know
about `Service` at all. Because of structural typing, `SQLiteRepo` can be
written months later, in a different file, by someone who's forgotten the
exact shape of `Service` - as long as method signatures line up, it slots in.

**CLI layer** (`main.go` + Kong): the only place that knows about *both*
sides. Picks which `Repository` implementation to construct, injects it into
`Service`.

```
core:            Job, Entry, Repository (interface), Service
implementations: TestRepo (in-memory), SQLiteRepo (real db)
cli:             constructs a Repository impl, wires it into Service, wraps
                 Service calls in Kong commands
```

## When to promote files -> packages
Packages enforce the separation (you can't accidentally reach into
`TestRepo` internals from `Service`), but add import-path ceremony. Stay
single-package/multi-file while `Repository`'s method set is still moving.
Once it stabilizes, promote implementations to their own packages
(e.g. `internal/storage/sqlite`).

## Gap to close before wiring up Kong
`plan.md`'s CLI surface is name-based (`track in <job>`, `track show <name>`),
but the current `Repository`/`Service` methods are ID-based only
(`ReadJob(id int)`). Define the exact `Service` method signatures the CLI
needs (name-based job lookup, clock in/out by name, status updates) before
building out `SQLiteRepo`, so the core is shaped by real call sites rather
than by what was convenient in the `TestRepo` scratch code.
