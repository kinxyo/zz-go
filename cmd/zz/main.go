package main

import (
	"github.com/kinxyo/zz-go/internal/cli"
	"github.com/kinxyo/zz-go/internal/collection"
	"github.com/kinxyo/zz-go/internal/config"
	"github.com/kinxyo/zz-go/internal/terminal"
	"github.com/kinxyo/zz-go/internal/tui"
)

//  ========================================================================================
//
//                 /$$
//                |__/
//       /$$$$$$$$ /$$  /$$$$$$  /$$$$$$$$  /$$$$$$   /$$$$$$
//      |____ /$$/| $$ /$$__  $$|____ /$$/ |____  $$ /$$__  $$
//         /$$$$/ | $$| $$  \ $$   /$$$$/   /$$$$$$$| $$  \ $$
//        /$$__/  | $$| $$  | $$  /$$__/   /$$__  $$| $$  | $$
//       /$$$$$$$$| $$|  $$$$$$$ /$$$$$$$$|  $$$$$$$|  $$$$$$$
//      |________/|__/ \____  $$|________/ \_______/ \____  $$
//                     /$$  \ $$                     /$$  \ $$
//                    |  $$$$$$/                    |  $$$$$$/
//                     \______/                      \______/
//
//        ⚡Version 1.0 ⚡
//
//  ========================================================================================

func main() {
	t := terminal.GetArgs()

	if *t.Flags.Help {
		printHelp()
		return
	}

	if len(t.Args) == 0 {
		tui.Run()
		return
	}

	if t.Args[0] == "run" {
		collection.Run(t.Args, &t.Flags)
		return
	}

	if t.Args[0] == "version" || t.Args[0] == "v" {
		printVersion()
		return
	}

	if t.Args[0] == "help" {
		printHelp()
		return
	}

	settings, err := config.Load()
	if err != nil {
		terminal.Err("config: %s (using defaults)\n", err)
	}

	cli.Run(t.Args, &t.Flags, settings)
}
