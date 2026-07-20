# time-tracker

A personal time tracker for jobs/projects: clock in, clock out, see where
the time went. A fast CLI for daily use, plus a local web UI for browsing
history and visual editing.

Written in Go, using [Kong](https://github.com/alecthomas/kong) for CLI
parsing, [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) for storage, and
[htmx](https://htmx.org/) for the web UI.

## Build

```sh
git clone https://github.com/JbrownWFU/time-tracker
cd time-tracker
go build -o track .
```

Or skip the build and run it directly:

```sh
go run . <command> [args]
```

`make build` / `make release` also work, and bake in version/commit/build
date (shown by `track about`) via `-ldflags`; they name the output binary
`time-tracker` rather than `track` (matches the Go module name). `make
release` cross-compiles for macOS (amd64/arm64), Linux (amd64), and
Windows (amd64) into `dist/`.

Requires Go 1.25+. No other setup — the database file and schema are
created automatically on first run.

## Usage

```
track [--db PATH] <command> [args]
```

`--db` is optional and defaults to `~/.tracker/time.db`, created
automatically on first run.

| Command | Description |
|---|---|
| `track create <name> [--desc TEXT] [--status todo\|active\|done]` | Create a new job (default status `todo`) |
| `track edit <name> [--name NEW] [--desc TEXT] [--status todo\|active\|done]` | Edit a job's name, description, and/or status |
| `track status <name> <todo\|active\|done>` | Shorthand for `edit <name> --status <status>` |
| `track delete <name> [--force]` | Delete a job and all its time spans (prompts for confirmation unless `--force`) |
| `track show <name>` | Print full details of a job |
| `track list` | List all jobs |
| `track in <job>` | Clock in to a job |
| `track out [notes] [--delete]` | Clock out of the current job, optionally attaching a note; `--delete` discards the open span instead of closing it |
| `track report <name> [--today\|--week\|--from DATE [--to DATE]] [--format text\|csv]` | Print a job's time spans (with notes) and a running total |
| `track where` | Print the currently clocked in job and duration to now |
| `track serve [port]` | Run the local web UI (default port `8283`) |
| `track about` | Print version, database path, and project URL |

## Example

```sh
./track create website --desc "personal site rebuild"
./track in website
# ... do some work ...
./track out "fixed the nav bar"
./track show website
```

`show`
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
./track report website --today
```

`report`
```
Report: website
[1] 2026-07-06 09:15 -> 2026-07-06 11:42	2h 27m
  note: fixed the nav bar
Total: 2h 27m
```

`report > CSV`
```sh
./track report website --format csv > website-report.csv
```

`delete`
```sh
./track delete website --force
```

## Web UI

```sh
./track serve
```

Opens a local server at `http://localhost:8283/ui` (or whatever port you
pass). It's the "admin console" side of this tool — the place for the
things the CLI deliberately doesn't do:

- **Click-to-edit, in place.** Both jobs (name/description/status) and
  individual spans (start time, end time, note) can be edited or deleted
  directly in the table — no separate page, no reload.
- **Clock in/out from the browser**, same one-job-at-a-time rule as the
  CLI. Every job row's state (which one's active, its running total)
  stays in sync live as you work, without polling.

## Report formats

`track report <name> --format csv` switches the output from the default
plain text to CSV (header row + one row per span, including a `notes`
column). Both formats include each span's ID, so you can identify a
specific entry — e.g. to go edit or delete it in the web UI.

Date filtering: `--today`, `--week`, or `--from DATE [--to DATE]`
(`YYYY-MM-DD`, and `--to` is inclusive). These are mutually exclusive with
each other; omit all of them to report on every span.

## Tips

- Only **one span can be open at a time, system-wide** — clock out (or
  `track out --delete` to undo) before clocking in to a different job.
- `track delete` removes a job and *all* of its time spans in one shot;
  pass `--force` to skip the `[y/N]` confirmation prompt (useful in
  scripts).
- The database is a plain SQLite file (`~/.tracker/time.db` by default).
  Back it up, copy it, or delete it to reset — `*.db` is already
  gitignored.

## Known limitations

- `track report`'s `--format` supports `text` and `csv`; JSON/Markdown
  formatters aren't built yet.
- The web UI's create-job form doesn't yet surface a server-side error
  (e.g. a duplicate name) — it just silently doesn't add the row.

## License

MIT — see [LICENSE](LICENSE).
