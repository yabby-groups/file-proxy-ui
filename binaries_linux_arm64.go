//go:build linux && arm64

package main

import "embed"

//go:embed all:bin/linux-arm64
var bundledBinaries embed.FS
