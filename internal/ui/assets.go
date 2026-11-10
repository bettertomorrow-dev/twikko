package ui

import "embed"

// Assets contains the compiled stylesheet and vendored htmx runtime.
//
//go:embed static/*
var Assets embed.FS
