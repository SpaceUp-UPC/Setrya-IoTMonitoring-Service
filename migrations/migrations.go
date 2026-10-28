// Package migrations embebe las migraciones SQL del servicio.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
