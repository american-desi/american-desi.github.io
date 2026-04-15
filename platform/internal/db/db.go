// Package db owns the SQLite connection pool, migrations, and repository-style
// accessors used by the rest of the platform.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps a *sql.DB with helper accessors.
type DB struct {
	*sql.DB
	path string
}

// Open opens the SQLite database at path (creating if needed), enables WAL,
// and applies any pending migrations.
func Open(path string) (*DB, error) {
	if path == "" {
		return nil, fmt.Errorf("db path is empty")
	}
	// Ensure the data directory exists.
	if err := ensureParentDir(path); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	sqldb.SetMaxOpenConns(8)
	sqldb.SetMaxIdleConns(4)
	sqldb.SetConnMaxLifetime(30 * time.Minute)

	if err := sqldb.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("sqlite ping: %w", err)
	}
	d := &DB{DB: sqldb, path: path}
	if err := d.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

// Path returns the underlying DB path.
func (d *DB) Path() string { return d.path }

// migrate applies any .sql migrations in lexical order that haven't been applied yet.
func (d *DB) migrate() error {
	// Ensure schema_migrations exists first (idempotent).
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		// Fallback: try disk path relative to cwd (useful when embed root doesn't match).
		return d.migrateFromDisk()
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		ver := extractVersion(name)
		if ver == 0 {
			continue
		}
		applied, err := d.migrationApplied(ver)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		raw, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if err := d.execMigration(ver, string(raw)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return nil
}

// migrateFromDisk is the fallback when embed didn't find the migrations (e.g.
// running `go test` from an unexpected cwd). It reads from ./data/migrations.
func (d *DB) migrateFromDisk() error {
	candidates := []string{"./data/migrations", "../data/migrations", "migrations"}
	var found string
	for _, c := range candidates {
		if entries, err := filepath.Glob(filepath.Join(c, "*.sql")); err == nil && len(entries) > 0 {
			found = c
			break
		}
	}
	if found == "" {
		return fmt.Errorf("no migrations found")
	}
	files, _ := filepath.Glob(filepath.Join(found, "*.sql"))
	sort.Strings(files)
	for _, f := range files {
		ver := extractVersion(filepath.Base(f))
		if ver == 0 {
			continue
		}
		applied, err := d.migrationApplied(ver)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		raw, err := readFile(f)
		if err != nil {
			return err
		}
		if err := d.execMigration(ver, raw); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) migrationApplied(ver int) (bool, error) {
	var n int
	row := d.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", ver)
	if err := row.Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

func (d *DB) execMigration(ver int, sqlText string) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(sqlText); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT OR IGNORE INTO schema_migrations (version) VALUES (?)", ver); err != nil {
		return err
	}
	return tx.Commit()
}

func extractVersion(filename string) int {
	// "0001_init.sql" -> 1
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) == 0 {
		return 0
	}
	var v int
	_, err := fmt.Sscanf(parts[0], "%d", &v)
	if err != nil {
		return 0
	}
	return v
}
