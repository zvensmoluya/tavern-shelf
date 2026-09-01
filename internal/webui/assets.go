package webui

import "embed"

// Assets contains the compiled Vue UI shipped in every Shelf binary.
//
//go:embed static/*
var Assets embed.FS
