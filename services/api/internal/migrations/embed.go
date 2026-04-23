// Package migrations embeds all goose SQL migration files into the binary.
// This allows a single self-contained binary to carry the full schema —
// no external migration directory required at runtime.
package migrations

import "embed"

// FS holds all *.sql files from this directory, embedded at compile time.
// Used by internal/db/migrator to run goose migrations via goose.SetBaseFS.
//
//go:embed *.sql
var FS embed.FS
