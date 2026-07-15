//go:build linux && amd64

package main

import "embed"

//go:embed all:bin/linux-amd64
var bundledBinaries embed.FS
