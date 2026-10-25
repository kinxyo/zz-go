package main

import (
	"os"

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
//        ⚡Version 1.2 ⚡
//
//  ========================================================================================

func main() {
	t := terminal.GetArgs()

	if *t.Flags.Help {
		terminal.PrintHelp()
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
		terminal.PrintVersion()
		return
	}

	if t.Args[0] == "help" {
		terminal.PrintHelp()
		return
	}

	wd, _ := os.Getwd() // "" if it can't be found: then no project entry is looked up
	conf, project, warnings := config.Load(wd)
	for _, w := range warnings {
		terminal.Err("config: %s\n", w)
	}

	cli.Run(t.Args, &t.Flags, conf, project)
}
