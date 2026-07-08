// Package migrations embeds the SQL migration files applied by
// internal/migrate at core startup.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
