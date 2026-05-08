// Package store wraps the SQLite database used for auth, sessions, audit, and
// the first-run setup token. Schema migrations live in store/migrations/ as
// numbered .sql files and are applied at Open() time.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB is a thin wrapper over *sql.DB that exposes typed accessors for each
// domain (users, sessions, audit, setup token).
type DB struct {
	*sql.DB
}

// Open dials SQLite at the given path, applies pending migrations, and returns
// a ready-to-use DB. WAL is enabled for better write concurrency; foreign keys
// are enforced; busy_timeout makes brief lock contention transparent.
func Open(path string) (*DB, error) {
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite is a single writer; cap connections to keep contention predictable.
	raw.SetMaxOpenConns(8)
	raw.SetMaxIdleConns(2)

	if err := raw.Ping(); err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	db := &DB{DB: raw}
	if err := db.migrate(); err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func (db *DB) migrate() error {
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at INTEGER NOT NULL
		);
	`); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)

	applied, err := db.appliedVersions(ctx)
	if err != nil {
		return err
	}

	for _, name := range files {
		version := strings.TrimSuffix(name, ".sql")
		if _, ok := applied[version]; ok {
			continue
		}
		body, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if err := db.applyMigration(ctx, version, body); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}
	return nil
}

func (db *DB) appliedVersions(ctx context.Context) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]struct{})
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = struct{}{}
	}
	return out, rows.Err()
}

func (db *DB) applyMigration(ctx context.Context, version string, body []byte) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, string(body)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, applied_at) VALUES (?, unixepoch())`,
		version,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// ErrNotFound is returned by typed accessors when a row is missing.
var ErrNotFound = errors.New("not found")

func wrapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
