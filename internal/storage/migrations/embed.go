package migrations

import "embed"

//go:embed postgres/*.sql mysql/*.sql sqlite/*.sql
var Files embed.FS
