package ui

import "embed"

// DistFS contains the embedded frontend assets from the dist directory.
//
//go:embed dist/*
var DistFS embed.FS
