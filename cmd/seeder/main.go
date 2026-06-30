// cmd/seeder/main.go — CLI untuk memasukkan data wilayah dari CSV ke database.
//
// Penggunaan:
//
//	go run ./cmd/seeder -file data/wilayah_indonesia.csv
//
// Idempoten: INSERT ... ON CONFLICT (kode_wilayah) DO NOTHING,
// sehingga aman dijalankan ulang tanpa duplikasi data.
// Jika kode_parent tidak ditemukan (data tidak konsisten), baris tetap
// dimasukkan dengan kode_parent = NULL dan dicetak sebagai peringatan.
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

	"github.com/bpskaltim/eform-backend/internal/config"
	"github.com/bpskaltim/eform-backend/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const batchSize = 500

const sqlInsert = `INSERT INTO wilayah (kode_wilayah, nama_wilayah, level, kode_parent)
                   VALUES ($1, $2, $3, $4) ON CONFLICT (kode_wilayah) DO NOTHING`

const sqlInsertNoParent = `INSERT INTO wilayah (kode_wilayah, nama_wilayah, level)
                           VALUES ($1, $2, $3) ON CONFLICT (kode_wilayah) DO NOTHING`

type wilayahRow struct {
	kode   string
	nama   string
	level  string
	parent any // nil untuk provinsi, string untuk level di bawahnya
}

func main() {
	csvPath := flag.String("file", "data/wilayah_indonesia.csv", "Path ke file CSV wilayah")
	flag.Parse()

	cfg := config.Load()
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("koneksi DB gagal: %v", err)
	}
	defer pool.Close()

	rows, err := readCSV(*csvPath)
	if err != nil {
		log.Fatalf("baca CSV: %v", err)
	}
	log.Printf("CSV dibaca: %d baris ditemukan", len(rows))

	// Urutkan berdasarkan panjang kode: provinsi(2) → kabupaten/kota(4) → kecamatan(7) → desa(10+)
	// agar FK kode_parent selalu tersedia sebelum child-nya diinsert.
	sort.Slice(rows, func(i, j int) bool {
		return len(rows[i].kode) < len(rows[j].kode)
	})

	inserted, skipped, orphaned := seedWilayah(ctx, pool, rows)

	fmt.Printf("\n✓ Selesai — %d baris baru, %d sudah ada, %d tanpa-parent\n",
		inserted, skipped, orphaned)
}

func readCSV(path string) ([]wilayahRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("buka file %q: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("baca header: %w", err)
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	colKode   := colIndex(idx, "kode_wilayah", 0)
	colNama   := colIndex(idx, "nama_wilayah", 1)
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
			return nil, fmt.Errorf("baris %d: %w", lineNum, err)
		}
		if len(rec) <= colKode || len(rec) <= colNama || len(rec) <= colLevel {
			log.Printf("[WARN] baris %d dilewati: kolom tidak lengkap", lineNum)
			continue
		}
		kode  := strings.TrimSpace(rec[colKode])
		nama  := strings.TrimSpace(rec[colNama])
		level := strings.TrimSpace(rec[colLevel])
		if kode == "" || nama == "" || level == "" {
			continue
		}
		var parent any
		if colParent < len(rec) {
			if p := strings.TrimSpace(rec[colParent]); p != "" {
				parent = p
			}
		}
		rows = append(rows, wilayahRow{kode, nama, level, parent})
	}
	return rows, nil
}

func colIndex(idx map[string]int, name string, fallback int) int {
	if i, ok := idx[name]; ok {
		return i
	}
	return fallback
}

// seedWilayah memasukkan data dalam batch. Jika satu batch gagal (misal FK violation
// karena data tidak konsisten), setiap baris dalam batch itu di-retry satu per satu.
// Baris yang masih gagal FK dimasukkan dengan kode_parent = NULL dan dicatat sebagai orphan.
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
			// Batch gagal → retry satu per satu
			for _, row := range chunk {
				ins, orp := insertOne(ctx, pool, row)
				inserted += ins
				skipped += (1 - ins - orp)
				orphaned += orp
			}
		}

		log.Printf("progress: %d/%d (%d baru, %d lewati, %d tanpa-parent)",
			end, total, inserted, skipped, orphaned)
	}
	return
}

// tryBatch mencoba memasukkan satu batch sekaligus. Mengembalikan false jika ada error.
func tryBatch(ctx context.Context, pool *pgxpool.Pool, chunk []wilayahRow) (inserted, skipped int, ok bool) {
	batch := &pgx.Batch{}
	for _, row := range chunk {
		batch.Queue(sqlInsert, row.kode, row.nama, row.level, row.parent)
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

// insertOne memasukkan satu baris. Jika FK violation, dicoba lagi tanpa parent.
// Mengembalikan (inserted=1, orphaned=0) normal, (1,1) orphan, (0,0) sudah ada.
func insertOne(ctx context.Context, pool *pgxpool.Pool, row wilayahRow) (inserted, orphaned int) {
	tag, err := pool.Exec(ctx, sqlInsert, row.kode, row.nama, row.level, row.parent)
	if err == nil {
		if tag.RowsAffected() > 0 {
			return 1, 0
		}
		return 0, 0 // sudah ada (conflict)
	}

	// Coba tanpa parent
	tag, err2 := pool.Exec(ctx, sqlInsertNoParent, row.kode, row.nama, row.level)
	if err2 != nil {
		log.Printf("[WARN] lewati %s (%s): %v", row.kode, row.nama, err2)
		return 0, 0
	}
	if tag.RowsAffected() > 0 {
		log.Printf("[WARN] orphan %s (%s): parent %v tidak ditemukan", row.kode, row.nama, row.parent)
		return 1, 1
	}
	return 0, 0 // sudah ada
}
