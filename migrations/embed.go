package migrations

import "embed"

// FS carries every .sql file for the migration runner to execute.
//
//go:embed *.sql
var FS embed.FS
