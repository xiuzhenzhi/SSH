package migrations

import "embed"

//go:embed postgres/*.sql mysql/*.sql
var Files embed.FS
