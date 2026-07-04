package main

import "embed"

//go:embed all:bin
var bundledBinaries embed.FS
