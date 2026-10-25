package terminal

import "github.com/kinxyo/zz-go/internal/config"

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
			{"get post put patch delete", "no method: get, even with a body"},
			{"head options", "show the status and headers only"},
			{"g p po pu pa d he opt o", "short forms"},
		},
	},
	{
		title: "BODY",
		hint:  "after the url; its Content-Type is set for you",
		width: 18,
		rows: []helpRow{
			{`'{"a":1}'`, "JSON: application/json"},
			{`'{a: 1}'`, `loose JSON, quoted for you: {"a": 1}`},
			{"'<a>...</a>'", "XML: application/xml"},
			{"@body.json", "read from a file (@~/ works), then the same as above"},
			{"other text", "text/plain (no body: no Content-Type)"},
		},
	},
	{
		title: "FLAGS",
		hint:  "anywhere: before or after the arguments",
		width: 18,
		rows: []helpRow{
			{`-H "Key: Value"`, "add a header (repeatable)"},
			{"-v", "show the config entry used, url, time, status and response headers"},
			{"-u  -t  -s  -i", "pick what to show: url, time, status, headers"},
			{"-p", "pretty-print a JSON body"},
			{"-f", "exit 22 on a 4xx or 5xx (the body is still printed)"},
			{"-d", "use http:// for urls typed without a scheme"},
			{"-r", "send the body as typed: no fix-ups, no Content-Type"},
			{"-m <seconds>", "time limit, 0 for none (default: the config's, 30)"},
		},
	},
	{
		title: "URLS",
		width: 18,
		rows: []helpRow{
			{"/users", "http://127.0.0.1:8000/users (host and port from the config)"},
			{"example.com/x", "https://example.com/x"},
			{"localhost:3000/x", "http://localhost:3000/x (so is 127.x.x.x and ::1)"},
			{"https://...", "used as typed"},
		},
	},
	{
		title: "EXIT",
		hint:  "full list in the README",
		rows: []helpRow{
			{"0 ok, 1 other failure, 2 bad arguments, 22 -f on 4xx/5xx, 23 binary not shown", ""},
			{"6 no such host, 7 refused, 28 timeout, 35 https to plain http, 60 bad cert", ""},
		},
	},
	{
		title: "CONFIG",
		rows: []helpRow{
			{`~/.config/zz/setting.json  {"host": "127.0.0.1", "port": 8000, "timeout": 30}`, ""},
			{`per directory and below: "projects": {"~/work/api": {"port": 3000}}`, ""},
		},
	},
}

// printHelp prints the usage text. It doesn't clear the screen, so it reads like any other command.
func PrintHelp() {
	PrintFC(TextBold, "zigzag")
	PrintF(" (zz) %s  quick HTTP requests from the terminal\n", config.Version)

	for _, s := range helpSections {
		PrintF("\n")
		PrintFC(TextBold, "%s", s.title)
		if s.hint != "" {
			PrintFC(TextDim, "  %s", s.hint)
		}
		PrintF("\n")

		for _, r := range s.rows {
			PrintF("  ")
			if r.right == "" {
				PrintFC(TextCyan, "%s", r.left)
			} else {
				PrintFC(TextCyan, "%-*s", s.width, r.left)
				PrintF("  %s", r.right)
			}
			PrintF("\n")
		}
	}
}
