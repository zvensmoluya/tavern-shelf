package webui

import "embed"

// Assets contains the dependency-free UI shipped in every Shelf binary.
//
//go:embed static/*
var Assets embed.FS
