package store

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/matheus3301/wpp/internal/store/migrations"
)

// MigrateResult describes what happened during migration.
type MigrateResult struct {
	Version uint
	Dirty   bool
	Changed bool
}

// Migrate runs all pending migrations on the database.
func (db *DB) Migrate() (*MigrateResult, error) {
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("migration source: %w", err)
	}

	driver, err := sqlite3.WithInstance(db.DB, &sqlite3.Config{})
	if err != nil {
		return nil, fmt.Errorf("migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "sqlite3", driver)
	if err != nil {
		return nil, fmt.Errorf("migration instance: %w", err)
	}

	err = m.Up()
	changed := true
	if err == migrate.ErrNoChange {
		changed = false
		err = nil
	}
	if err != nil {
		return nil, fmt.Errorf("migration up: %w", err)
	}

	version, dirty, _ := m.Version()
	return &MigrateResult{
		Version: version,
		Dirty:   dirty,
		Changed: changed,
	}, nil
}
