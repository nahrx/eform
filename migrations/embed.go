package migrations

import "embed"

// FS memuat seluruh file .sql untuk dijalankan oleh migration runner.
//
//go:embed *.sql
var FS embed.FS
