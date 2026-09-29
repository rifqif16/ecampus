package dbfs

import "embed"

const MigrationsDir = "migrations"

//go:embed migrations/*.sql
var Migrations embed.FS
