// Package migration runs versioned schema migrations at service startup using
// golang-migrate with SQL files embedded into the binary. It replaces gorm
// AutoMigrate, which cannot express the CDC prerequisites (REPLICA IDENTITY,
// PUBLICATION) that Debezium needs. golang-migrate takes a Postgres advisory
// lock while running, so it is safe to call from multiple replicas concurrently.
package migration

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	pgxmig "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

//go:embed sql/*.sql
var files embed.FS

// Run applies all pending up migrations against the given DSN.
func Run(dsn string, log *slog.Logger) error {
	src, err := iofs.New(files, "sql")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open migration db: %w", err)
	}
	defer db.Close()

	drv, err := pgxmig.WithInstance(db, &pgxmig.Config{})
	if err != nil {
		return fmt.Errorf("init migrate driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "pgx", drv)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	version, _, _ := m.Version()
	log.Info("migrations applied (product_db)", "version", version)
	return nil
}
