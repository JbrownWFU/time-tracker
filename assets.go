package main

import "embed"

// Embedded so the built binary is self-contained - no static/ or
// templates/ directory needs to ship alongside it. go:embed requires the
// directive to live in the same directory tree as the embedded files,
// which is why this is at the repo root (package main) rather than in
// src/server, even though the server package is what actually uses it.
//
//go:embed static templates
var assets embed.FS
