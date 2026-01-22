package envoy

import "embed"

//go:embed web/templates/*
var webFS embed.FS
