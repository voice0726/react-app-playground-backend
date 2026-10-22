package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/voice0726/react-app-playground-backend/internal/config"
	"github.com/voice0726/react-app-playground-backend/internal/platform/database"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if err := config.LoadDotEnv(); err != nil {
		return err
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}
	if len(args) == 0 {
		return errors.New("usage: migrate up|down 1|version")
	}
	connectionTimeout, err := database.ConnectionTimeout()
	if err != nil {
		return err
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	err = database.WaitForConnection(ctx, db, slog.Default())
	cancel()
	if err != nil {
		return err
	}
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithDatabaseInstance("file://migrations", "playground", driver)
	if err != nil {
		return err
	}
	switch args[0] {
	case "up":
		err = m.Up()
	case "down":
		if len(args) != 2 || args[1] != "1" {
			return errors.New("usage: migrate down 1")
		}
		err = m.Steps(-1)
	case "version":
		var version uint
		var dirty bool
		version, dirty, err = m.Version()
		if err == nil {
			fmt.Printf("version=%d dirty=%t\n", version, dirty)
		}
	default:
		return errors.New("usage: migrate up|down 1|version")
	}
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}
