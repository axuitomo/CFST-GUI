package main

import "embed"

//go:embed all:frontend/dist
var frontendAssets embed.FS
