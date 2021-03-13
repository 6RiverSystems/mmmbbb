package migrations

import "embed"

//go:embed message-bus/*.sql
var MessageBusMigrations embed.FS
