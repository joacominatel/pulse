package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joacominatel/pulse/internal/infrastructure/logging"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migration represents a single database migration.
type Migration struct {
	Version     string
	Description string
	UpSQL       string
	DownSQL     string
}

// Migrator handles database migrations.
type Migrator struct {
	pool   *pgxpool.Pool
	schema string
	logger *logging.Logger
}

// NewMigrator creates a new migrator instance.
func NewMigrator(conn *Connection, logger *logging.Logger) *Migrator {
	return &Migrator{
		pool:   conn.Pool(),
		schema: conn.Schema(),
		logger: logger.WithComponent("migrator"),
	}
}

// Run applies all pending migrations.
func (m *Migrator) Run(ctx context.Context) error {
	m.logger.MigrationStarted()

	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("loading migrations: %w", err)
	}

	appliedCount := 0
	for _, migration := range migrations {
		applied, err := m.applyMigration(ctx, migration)
		if err != nil {
			m.logger.MigrationFailed(migration.Version, migration.Description, err)
			return fmt.Errorf("applying migration %s: %w", migration.Version, err)
		}
		if applied {
			appliedCount++
		}
	}

	m.logger.MigrationCompleted(appliedCount)
	return nil
}

// loadMigrations reads all migration files from the embedded filesystem.
func (m *Migrator) loadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("reading migrations directory: %w", err)
	}

	// group .up.sql and .down.sql files by version
	migrationMap := make(map[string]*Migration)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		// parse filename: 000001_description.up.sql or 000001_description.down.sql
		var version, description, direction string
		switch {
		case strings.HasSuffix(name, ".up.sql"):
			direction = "up"
			base := strings.TrimSuffix(name, ".up.sql")
			parts := strings.SplitN(base, "_", 2)
			if len(parts) != 2 {
				continue
			}
			version = parts[0]
			description = parts[1]
		case strings.HasSuffix(name, ".down.sql"):
			direction = "down"
			base := strings.TrimSuffix(name, ".down.sql")
			parts := strings.SplitN(base, "_", 2)
			if len(parts) != 2 {
				continue
			}
			version = parts[0]
			description = parts[1]
		default:
			continue
		}

		// embed.FS always uses forward slash regardless of OS
		content, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			return nil, fmt.Errorf("reading migration file %s: %w", name, err)
		}

		if _, exists := migrationMap[version]; !exists {
			migrationMap[version] = &Migration{
				Version:     version,
				Description: description,
			}
		}

		if direction == "up" {
			migrationMap[version].UpSQL = string(content)
		} else {
			migrationMap[version].DownSQL = string(content)
		}
	}

	// sort migrations by version
	var migrations []Migration
	for _, mig := range migrationMap {
		if mig.UpSQL != "" { // only include migrations with up scripts
			migrations = append(migrations, *mig)
		}
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// applyMigration applies a single migration if not already applied.
// returns true if migration was applied, false if already applied.
func (m *Migrator) applyMigration(ctx context.Context, migration Migration) (bool, error) {
	// check if already applied
	var exists bool
	err := m.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pulse.schema_migrations WHERE version = $1)`,
		migration.Version,
	).Scan(&exists)

	// if schema_migrations doesn't exist yet, first migration will create it
	if err != nil && migration.Version != "000001" {
		return false, fmt.Errorf("checking migration status: %w", err)
	}

	if exists {
		m.logger.MigrationSkipped(migration.Version, migration.Description)
		return false, nil
	}

	// apply migration in a transaction
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// execute migration SQL
	if _, err := tx.Exec(ctx, migration.UpSQL); err != nil {
		return false, fmt.Errorf("executing migration: %w", err)
	}

	// record migration in schema_migrations table
	if _, err := tx.Exec(ctx,
		`INSERT INTO pulse.schema_migrations (version, description) VALUES ($1, $2)`,
		migration.Version, migration.Description,
	); err != nil {
		return false, fmt.Errorf("recording migration: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("committing transaction: %w", err)
	}

	m.logger.MigrationApplied(migration.Version, migration.Description)
	return true, nil
}

// GetAppliedMigrations returns a list of applied migration versions.
func (m *Migrator) GetAppliedMigrations(ctx context.Context) ([]string, error) {
	rows, err := m.pool.Query(ctx,
		`SELECT version FROM pulse.schema_migrations ORDER BY version`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying migrations: %w", err)
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scanning version: %w", err)
		}
		versions = append(versions, version)
	}

	return versions, rows.Err()
}
