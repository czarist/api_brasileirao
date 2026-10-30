// Package migrations embeds the forward-only SQL migrations.
package migrations

import "embed"

// FS contains the versioned SQL files. Applied files must not be edited.
//
//go:embed *.sql
var FS embed.FS
