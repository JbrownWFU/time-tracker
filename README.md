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

`--db` is optional and defaults to `~/.tracker/time.db`, created automatically
on first run.

| Command | Description |
|---|---|
| `track create <name> [--desc TEXT] [--status todo\|active\|done]` | Create a new job (default status `todo`) |
| `track edit <name> [--name NEW] [--desc TEXT] [--status todo\|active\|done]` | Edit a job's name, description, and/or status |
| `track status <name> <todo\|active\|done>` | Shorthand for `edit <name> --status <status>` |
| `track delete <name> [--force]` | Delete a job and all its time spans (prompts for confirmation unless `--force`) |
| `track show <name>` | Print full details of a job |
| `track list` | List all jobs |
| `track in <job>` | Clock in to a job |
| `track out <job> [notes]` | Clock out of a job, optionally attaching notes |
| `track report <name> [--file\|-o PATH]` | Print time entries for a job with a running total, or export to a file |

## Example

```sh
./track create website --desc "personal site rebuild"
./track in website
# ... do some work ...
./track out website "fixed the nav bar"
./track show website
```

```
Name:         website
Status:       todo
Description:  personal site rebuild
Time entries: 1
Total time:   2h 27m
Clocked in:   no
```

```sh
./track edit website --status active
./track report website
```

```
Time entries for "website":
2026-07-06 09:15 -> 2026-07-06 11:42	2h 27m
Total: 2h 27m
```

```sh
./track report website --file website-report.csv
./track delete website --force
```

## Report export

`track report <name> --file PATH` (or `-o PATH`) writes the report to a file
instead of stdout. The format is inferred from the file extension:

- `.csv` — header row (`start,end,duration`) followed by one row per entry
- `.md` — a GitHub-flavored markdown table
- anything else — plain text, one `start -> end	duration` line per entry

## Tips

- Only **one span can be open at a time, system-wide** — clock out before
  clocking in to a different job. `track out` also errors if the currently
  open span belongs to a different job than the one you named.
- `track delete` removes a job and *all* of its time spans in one shot; pass
  `--force` to skip the `[y/N]` confirmation prompt (useful in scripts).
- The database is a plain SQLite file (`~/.tracker/time.db` by default). Back
  it up, copy it, or delete it to reset — `*.db` is already gitignored.

## Known limitations

- Notes attached via `track out <job> "some note"` are stored but can't yet
  be viewed from the CLI.
- No command yet to edit or delete an individual time span (clock-in/out
  entry) — only whole jobs (`track delete`) can be removed.
