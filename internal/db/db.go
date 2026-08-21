package db

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// envInt reads a positive integer from the environment, falling back to def.
func envInt(key string, def int32) int32 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		log.Printf("[db] %s=%q is not a positive number — using %d", key, v, def)
		return def
	}
	return int32(n)
}

// Connect opens the pool and waits for the database to become ready (retrying a few times).
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("pool configuration: %w", err)
	}

	/* Left unset, pgx allows max(4, NumCPU) connections. On a small VPS that is four:
	   every request that touches the database queues behind three others, so a handful
	   of simultaneous users is enough to turn a 100 ms query into a multi-second wait,
	   with nothing in the logs to show for it.

	   Raise this together with PostgreSQL's own max_connections (default 100), and
	   divide it by the number of application instances pointing at the same database —
	   the limit is shared. */
	cfg.MaxConns = envInt("DB_MAX_CONNS", 25)
	// A few connections kept ready, so the first requests after an idle period do not
	// each pay for a TCP handshake and authentication.
	cfg.MinConns = envInt("DB_MIN_CONNS", 2)
	// Recycled periodically: a long-lived connection holds server-side memory and
	// survives failovers it should not.
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pool configuration: %w", err)
	}
	log.Printf("[db] connection pool: max=%d min=%d", cfg.MaxConns, cfg.MinConns)
	var lastErr error
	for i := 0; i < 10; i++ {
		ctxPing, cancel := context.WithTimeout(ctx, 3*time.Second)
		lastErr = pool.Ping(ctxPing)
		cancel()
		if lastErr == nil {
			return pool, nil
		}
		log.Printf("[db] waiting for PostgreSQL to become ready (%d/10): %v", i+1, lastErr)
		time.Sleep(2 * time.Second)
	}
	pool.Close()
	return nil, fmt.Errorf("cannot connect to PostgreSQL: %w", lastErr)
}

// Migrate runs every *.up.sql file that has not been applied yet, in order, each in
// its own transaction, and records it in the schema_migrations table.
func Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied := map[string]bool{}
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return err
	}
	var ups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			ups = append(ups, e.Name())
		}
	}
	sort.Strings(ups)

	count := 0
	for _, name := range ups {
		version := strings.TrimSuffix(name, ".up.sql")
		if applied[version] {
			continue
		}
		sqlBytes, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s failed: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		log.Printf("[migrate] applied: %s", version)
		count++
	}
	if count == 0 {
		log.Println("[migrate] no new migrations")
	}
	return nil
}
