package migrations

import "embed"

// Files exposes embedded SQL migration files ordered lexicographically.
//
//go:embed *.sql sqlite
var Files embed.FS
