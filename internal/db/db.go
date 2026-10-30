// Package db connects to PostgreSQL and applies schema migrations.
package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Connect opens a connection pool, waiting up to timeout for the database to
// accept connections (the database container may still be starting).
func Connect(ctx context.Context, dsn string, timeout time.Duration) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("database settings: %w", err)
	}
	cfg.MaxConns = 20
	deadline := time.Now().Add(timeout)
	for {
		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				return pool, nil
			}
			pool.Close()
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("database not reachable: %w", err)
		}
		log.Printf("waiting for database: %v", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

type migration struct {
	version int
	name    string
	sql     string
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	var ms []migration
	for _, e := range entries {
		n := e.Name()
		num, rest, ok := strings.Cut(n, "_")
		if !ok || !strings.HasSuffix(n, ".sql") {
			continue
		}
		v, err := strconv.Atoi(num)
		if err != nil {
			return nil, fmt.Errorf("migration %s: bad number", n)
		}
		b, err := migrationFiles.ReadFile("migrations/" + n)
		if err != nil {
			return nil, err
		}
		ms = append(ms, migration{v, strings.TrimSuffix(rest, ".sql"), string(b)})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].version < ms[j].version })
	return ms, nil
}

// Migrate applies pending migrations in order, each in its own transaction,
// and records each one in the audit log. A lock prevents two servers from
// migrating at once.
func Migrate(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	ms, err := loadMigrations()
	if err != nil {
		return nil, err
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(727001)`); err != nil {
		return nil, err
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock(727001)`)
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version integer PRIMARY KEY,
		name text NOT NULL,
		applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return nil, err
	}
	applied := map[int]bool{}
	rows, err := conn.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return nil, err
		}
		applied[v] = true
	}
	rows.Close()
	var done []string
	for _, m := range ms {
		if applied[m.version] {
			continue
		}
		err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, m.sql); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, m.version, m.name); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO audit_log (action, detail) VALUES ('schema.migrate', jsonb_build_object('version', $1::int, 'name', $2::text))`, m.version, m.name)
			return err
		})
		if err != nil {
			return done, fmt.Errorf("migration %04d_%s: %w", m.version, m.name, err)
		}
		done = append(done, fmt.Sprintf("%04d_%s", m.version, m.name))
	}
	return done, nil
}

// ErrNotFound is returned by lookups that find no row.
var ErrNotFound = errors.New("not found")

// NotFound converts pgx.ErrNoRows to ErrNotFound.
func NotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
