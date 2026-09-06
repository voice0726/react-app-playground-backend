package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/voice0726/react-app-playground-backend/internal/config"
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
	db, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer db.Close()
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
