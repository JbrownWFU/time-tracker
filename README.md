# time-tracker

A personal CLI for tracking time spent on jobs/projects. Written in Go, uses
[Kong](https://github.com/alecthomas/kong) for argument parsing and
[modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (a pure-Go SQLite
driver, no CGo needed) for storage.

## Build

```sh
git clone https://github.com/JbrownWFU/time-tracker
cd time-tracker
go build -o track .
```

Or skip the build and just run it directly:

```sh
go run . <command> [args]
```

Requires Go 1.25.3+. No other setup — the database file and schema are
created automatically on first run.

## Usage

```
track [--db PATH] <command> [args]
```

`--db` is optional and defaults to `time.db` in the current directory.

| Command | Description |
|---|---|
| `track create <name> [--desc TEXT] [--status todo\|active\|done]` | Create a new job (default status `todo`) |
| `track status <name> <todo\|active\|done>` | Update a job's status |
| `track show <name>` | Print full details of a job |
| `track list` | List all jobs |
| `track in <job>` | Clock in to a job |
| `track out <job> [notes]` | Clock out of a job, optionally attaching notes |
| `track report <name>` | Print all time entries for a job with a running total |

## Example

```sh
./track create website --desc "personal site rebuild"
./track in website
# ... do some work ...
./track out website "fixed the nav bar"
./track report website
```

```
Time entries for "website":
2026-07-06 09:15 -> 2026-07-06 11:42	2h 27m
Total: 2h 27m
```

## Tips

- Only **one span can be open at a time, system-wide** — clock out before
  clocking in to a different job. `track out` also errors if the currently
  open span belongs to a different job than the one you named.
- The database is a plain SQLite file (`time.db` by default). Back it up,
  copy it, or delete it to reset — `*.db` is already gitignored.
- Output is still pretty basic (`show` just dumps the struct, `list` is
  tab-separated) — see [TODO.md](TODO.md) for known rough edges.
