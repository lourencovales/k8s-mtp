package store

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"git.assilvestrar.club/lourenco/k8s-mtp/internal/config"
)

type Database struct {
	db     *sql.DB
	logger *slog.Logger
}

func New(cfg *config.Config, logger *slog.Logger) (*Database, error) {
	db, err := sql.Open("postgres", cfg.DBURL)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}

	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)

	if cfg.DBConnMaxLifetime != "" {
		duration, err := time.ParseDuration(cfg.DBConnMaxLifetime)
		if err != nil {
			return nil, fmt.Errorf("invalid DB connection max lifetime: %w", err)
		}
		db.SetConnMaxLifetime(duration)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("cannot ping database: %w", err)
	}

	return &Database{
		db:     db,
		logger: logger,
	}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) RunMigrations(cfg *config.Config, migrationPath string) error {
	driver, err := postgres.WithInstance(d.db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		migrationPath,
		"postgres", driver,
	)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return m.Up()
}

func NewTestDB(rawDB *sql.DB, logger *slog.Logger) *Database {
	return &Database{db: rawDB, logger: logger}
}
