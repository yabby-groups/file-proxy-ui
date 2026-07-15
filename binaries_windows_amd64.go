//go:build windows && amd64

package main

import "embed"

//go:embed all:bin/windows-amd64
var bundledBinaries embed.FS
