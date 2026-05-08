package main

import (
	"database/sql"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/solanyn/mono/line/migrations"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("DATABASE_URL environment variable required")
		os.Exit(1)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		slog.Error("failed to ping database", "err", err)
		os.Exit(1)
	}

	goose.SetBaseFS(migrations.FS)

	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	slog.Info("running migrations", "command", cmd)

	var cmdErr error
	switch cmd {
	case "up":
		cmdErr = goose.Up(db, ".")
	case "down":
		cmdErr = goose.Down(db, ".")
	case "status":
		cmdErr = goose.Status(db, ".")
	case "reset":
		cmdErr = goose.Reset(db, ".")
	default:
		slog.Error("unknown command", "cmd", cmd)
		os.Exit(1)
	}

	if cmdErr != nil {
		slog.Error("migration failed", "cmd", cmd, "err", cmdErr)
		os.Exit(1)
	}

	slog.Info("migrations complete", "command", cmd)
}
