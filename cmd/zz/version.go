package main

import "github.com/kinxyo/zz-go/internal/terminal"

// version can be overridden at build time:
//
//	go build -ldflags "-X main.version=1.2.3" ./cmd/zz
var version = "1.2"

// printVersion prints the program name and version.
func printVersion() {
	terminal.Print("zz -- v" + version)
}
