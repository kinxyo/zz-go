package main

import (
	"github.com/kinxyo/zz-go/internal/cli"
	"github.com/kinxyo/zz-go/internal/collection"
	"github.com/kinxyo/zz-go/internal/terminal"
	"github.com/kinxyo/zz-go/internal/tui"
)

//  ========================================================================================
//
//      $$$$$$$$\ $$$$$$\   $$$$$$\
//      \____$$  |\____$$\ $$  __$$\
//        $$$$ _/ $$$$$$$ |$$ /  $$ |
//       $$  _/  $$  __$$ |$$ |  $$ |
//      $$$$$$$$\\$$$$$$$ |$$$$$$$  |
//      \________|\_______|$$  ____/
//                         $$ |
//                         $$ |
//                         \__|
//
//        ⚡Version 0.6 ⚡
//
//  ========================================================================================

func main() {
	flags := terminal.LoadFlags()
	args := terminal.LoadArgs()

	// No args
	if len(args) == 0 {
		tui.Run()
		return
	}

	// 1st arg "run"
	if args[0] == "run" {
		collection.Run(args, flags)
		return
	}

	if args[0] == "version" || args[0] == "v" {
		terminal.Print("zap -- v0.5")
		return
	}

	// Anything else (No arg case already handled!)
	cli.Run(args, flags)
}
