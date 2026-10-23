package main

import "github.com/kinxyo/zz-go/internal/terminal"

type helpRow struct {
	left  string // command, flag or example, shown in cyan
	right string // what it does; empty for a plain list
}

type helpSection struct {
	title string
	hint  string // dim note after the title
	width int    // width of the left column
	rows  []helpRow
}

var helpSections = []helpSection{
	{
		title: "USAGE",
		rows: []helpRow{
			{"zz [flags] [method] <url> [body]", ""},
			{"zz <command>", ""},
		},
	},
	{
		title: "COMMANDS",
		width: 18,
		rows: []helpRow{
			{"help, -h", "show this help"},
			{"version, v", "show the version"},
			{"run [file]", "read a .zz collection (requests are not sent yet)"},
		},
	},
	{
		title: "METHODS",
		width: 30,
		rows: []helpRow{
			{"get post put patch delete", "no method: get, or post with a body"},
			{"head options", "show the status and headers only"},
			{"g p po pu pa d he opt o", "short forms"},
		},
	},
	{
		title: "BODY",
		hint:  "the last argument",
		width: 18,
		rows: []helpRow{
			{`'{"a":1}'`, "inline JSON; Content-Type: application/json is added"},
			{"@body.json", "read the body from a file"},
			{"-", "read the body from stdin"},
			{"anything else", `needs your own -H "Content-Type: ..."`},
		},
	},
	{
		title: "FLAGS",
		hint:  "go before the url",
		width: 18,
		rows: []helpRow{
			{`-H "Key: Value"`, "add a header (repeatable)"},
			{"-v", "show url, time, status and response headers"},
			{"-u  -t  -s  -i", "pick what to show: url, time, status, headers"},
			{"-p", "pretty-print a JSON body"},
			{"-f", "exit 22 on a 4xx or 5xx (the body is still printed)"},
			{"-d", "use http:// for urls typed without a scheme"},
		},
	},
	{
		title: "URLS",
		width: 18,
		rows: []helpRow{
			{"/users", "http://127.0.0.1:8000/users (host and port from the config)"},
			{"example.com/x", "https://example.com/x"},
			{"https://...", "used as typed"},
		},
	},
	{
		title: "CONFIG",
		rows: []helpRow{
			{`~/.config/zz/setting.json  {"host": "127.0.0.1", "port": 8000}`, ""},
		},
	},
}

// printHelp prints the usage text. It doesn't clear the screen, so it reads like any other command.
func printHelp() {
	terminal.PrintFC(terminal.TextBold, "zigzag")
	terminal.PrintF(" (zz) %s  quick HTTP requests from the terminal\n", version)

	for _, s := range helpSections {
		terminal.PrintF("\n")
		terminal.PrintFC(terminal.TextBold, "%s", s.title)
		if s.hint != "" {
			terminal.PrintFC(terminal.TextDim, "  %s", s.hint)
		}
		terminal.PrintF("\n")

		for _, r := range s.rows {
			terminal.PrintF("  ")
			if r.right == "" {
				terminal.PrintFC(terminal.TextCyan, "%s", r.left)
			} else {
				terminal.PrintFC(terminal.TextCyan, "%-*s", s.width, r.left)
				terminal.PrintF("  %s", r.right)
			}
			terminal.PrintF("\n")
		}
	}
}
