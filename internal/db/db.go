package db

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens the pool and waits for the database to become ready (retrying a few times).
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pool configuration: %w", err)
	}
	var lastErr error
	for i := 0; i < 10; i++ {
		ctxPing, cancel := context.WithTimeout(ctx, 3*time.Second)
		lastErr = pool.Ping(ctxPing)
		cancel()
		if lastErr == nil {
			return pool, nil
		}
		log.Printf("[db] menunggu PostgreSQL siap (%d/10): %v", i+1, lastErr)
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
