// cmd/seeder/main.go — CLI that loads region data from a CSV into the database.
//
// Usage:
//
//	go run ./cmd/seeder -file data/wilayah_indonesia.csv
//
// Idempoten: INSERT ... ON CONFLICT (kode_wilayah) DO NOTHING,
// so it is safe to re-run without duplicating data.
// When kode_parent is missing (inconsistent data), the row is still inserted
// with kode_parent = NULL and reported as a warning.
package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/nahrx/eform/internal/config"
	"github.com/nahrx/eform/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const batchSize = 500

const sqlInsert = `INSERT INTO wilayah (kode_wilayah, nama_wilayah, level, kode_parent)
                   VALUES ($1, $2, $3, $4) ON CONFLICT (kode_wilayah) DO NOTHING`

const sqlInsertNoParent = `INSERT INTO wilayah (kode_wilayah, nama_wilayah, level)
                           VALUES ($1, $2, $3) ON CONFLICT (kode_wilayah) DO NOTHING`

type wilayahRow struct {
	code   string
	name   string
	level  string
	parent any // nil for provinces, string for the levels below
}

func main() {
	csvPath := flag.String("file", "data/wilayah_indonesia.csv", "Path ke file CSV wilayah")
	flag.Parse()

	cfg := config.Load()
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	defer pool.Close()

	rows, err := readCSV(*csvPath)
	if err != nil {
		log.Fatalf("read CSV: %v", err)
	}
	log.Printf("CSV read: %d rows found", len(rows))

	// Sort by code length: province(2) -> regency/city(4) -> district(7) -> village(10+)
	// so the kode_parent foreign key always exists before its children are inserted.
	sort.Slice(rows, func(i, j int) bool {
		return len(rows[i].code) < len(rows[j].code)
	})

	inserted, skipped, orphaned := seedWilayah(ctx, pool, rows)

	fmt.Printf("\n✓ Done — %d new rows, %d already present, %d without parent\n",
		inserted, skipped, orphaned)
}

func readCSV(path string) ([]wilayahRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file %q: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	colKode   := colIndex(idx, "kode_wilayah", 0)
	colName   := colIndex(idx, "nama_wilayah", 1)
	colLevel  := colIndex(idx, "level", 2)
	colParent := colIndex(idx, "kode_parent", 3)

	var rows []wilayahRow
	lineNum := 1
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		lineNum++
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", lineNum, err)
		}
		if len(rec) <= colKode || len(rec) <= colName || len(rec) <= colLevel {
			log.Printf("[WARN] row %d skipped: incomplete columns", lineNum)
			continue
		}
		code  := strings.TrimSpace(rec[colKode])
		name  := strings.TrimSpace(rec[colName])
		level := strings.TrimSpace(rec[colLevel])
		if code == "" || name == "" || level == "" {
			continue
		}
		var parent any
		if colParent < len(rec) {
			if p := strings.TrimSpace(rec[colParent]); p != "" {
				parent = p
			}
		}
		rows = append(rows, wilayahRow{code, name, level, parent})
	}
	return rows, nil
}

func colIndex(idx map[string]int, name string, fallback int) int {
	if i, ok := idx[name]; ok {
		return i
	}
	return fallback
}

// seedWilayah inserts rows in batches. If a batch fails (an FK violation from inconsistent
// data, for instance), every row in that batch is retried individually.
// Rows that still fail the FK check are inserted with kode_parent = NULL and recorded as orphans.
func seedWilayah(ctx context.Context, pool *pgxpool.Pool, rows []wilayahRow) (inserted, skipped, orphaned int) {
	total := len(rows)

	for start := 0; start < total; start += batchSize {
		end := start + batchSize
		if end > total {
			end = total
		}
		chunk := rows[start:end]

		ins, skip, ok := tryBatch(ctx, pool, chunk)
		if ok {
			inserted += ins
			skipped += skip
		} else {
			// Batch failed → retry row by row
			for _, row := range chunk {
				ins, orp := insertOne(ctx, pool, row)
				inserted += ins
				skipped += (1 - ins - orp)
				orphaned += orp
			}
		}

		log.Printf("progress: %d/%d (%d new, %d skipped, %d without parent)",
			end, total, inserted, skipped, orphaned)
	}
	return
}

// tryBatch attempts to insert one whole batch. It returns false if anything fails.
func tryBatch(ctx context.Context, pool *pgxpool.Pool, chunk []wilayahRow) (inserted, skipped int, ok bool) {
	batch := &pgx.Batch{}
	for _, row := range chunk {
		batch.Queue(sqlInsert, row.code, row.name, row.level, row.parent)
	}

	br := pool.SendBatch(ctx, batch)
	defer br.Close()

	for range chunk {
		tag, err := br.Exec()
		if err != nil {
			return 0, 0, false
		}
		if tag.RowsAffected() > 0 {
			inserted++
		} else {
			skipped++
		}
	}
	return inserted, skipped, true
}

// insertOne inserts a single row. On an FK violation it retries without the parent.
// Returns (inserted=1, orphaned=0) normally, (1,1) for an orphan, (0,0) if it already exists.
func insertOne(ctx context.Context, pool *pgxpool.Pool, row wilayahRow) (inserted, orphaned int) {
	tag, err := pool.Exec(ctx, sqlInsert, row.code, row.name, row.level, row.parent)
	if err == nil {
		if tag.RowsAffected() > 0 {
			return 1, 0
		}
		return 0, 0 // already exists (conflict)
	}

	// Retry without the parent
	tag, err2 := pool.Exec(ctx, sqlInsertNoParent, row.code, row.name, row.level)
	if err2 != nil {
		log.Printf("[WARN] lewati %s (%s): %v", row.code, row.name, err2)
		return 0, 0
	}
	if tag.RowsAffected() > 0 {
		log.Printf("[WARN] orphan %s (%s): parent %v not found", row.code, row.name, row.parent)
		return 1, 1
	}
	return 0, 0 // already exists
}
