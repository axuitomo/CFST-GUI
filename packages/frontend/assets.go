package frontendassets

import "embed"

// Dist embeds the built frontend static distribution bundle (or placeholder .gitkeep before build).
//
//go:embed all:dist
var Dist embed.FS
