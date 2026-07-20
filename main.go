package main

import (
	"os"
	"path/filepath"

	SqlDB "time-tracker/src/sqldb"

	"github.com/alecthomas/kong"
)

// Set via -ldflags at build time; see Makefile.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const githubURL = "https://github.com/JbrownWFU/time-tracker"

// defaultDBPath returns ~/.tracker/time.db, creating the .tracker directory if needed.
func defaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".tracker")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "time.db"), nil
}

func main() {
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "--help")
	}

	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("track"),
		kong.Description("A simple time tracking CLI."),
		kong.UsageOnError(),
	)

	if cli.DB == "" {
		path, err := defaultDBPath()
		if err != nil {
			ctx.Fatalf("failed to resolve default db path: %v", err)
		}
		cli.DB = path
	}

	db, err := SqlDB.NewSqlConn(cli.DB)
	if err != nil {
		ctx.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	err = ctx.Run(&db)
	ctx.FatalIfErrorf(err)
}
