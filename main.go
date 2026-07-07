package main

import (
	"github.com/alecthomas/kong"
	SqlDB "time-tracker/src"
)

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("track"),
		kong.Description("A simple time tracking CLI."),
		kong.UsageOnError(),
	)

	db, err := SqlDB.NewSqlConn(cli.DB)
	if err != nil {
		ctx.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	err = ctx.Run(&db)
	ctx.FatalIfErrorf(err)
}
