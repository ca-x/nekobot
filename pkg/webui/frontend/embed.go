package frontend

import "embed"

// Dist embeds the compiled frontend assets.
// Build the frontend first: cd frontend && npm run build
// The built assets go into frontend/dist/
//
//go:embed dist/*
var Dist embed.FS
