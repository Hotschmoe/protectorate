package envoy

import "embed"

//go:embed templates/* static/*
var WebFS embed.FS
