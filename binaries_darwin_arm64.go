//go:build darwin && arm64

package main

import "embed"

//go:embed all:bin/darwin-arm64
var bundledBinaries embed.FS
