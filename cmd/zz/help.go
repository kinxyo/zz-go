package main

import "github.com/kinxyo/zz-go/internal/terminal"

// printHelp clears the screen and prints the usage text.
func printHelp() {
	terminal.Clear()
	terminal.PrintF("%s", "\n"+logo+"\n                      Version "+version+"\n\n"+body)
}

const logo = `            /$$
           |__/
  /$$$$$$$$ /$$  /$$$$$$  /$$$$$$$$  /$$$$$$   /$$$$$$
 |____ /$$/| $$ /$$__  $$|____ /$$/ |____  $$ /$$__  $$
    /$$$$/ | $$| $$  \ $$   /$$$$/   /$$$$$$$| $$  \ $$
   /$$__/  | $$| $$  | $$  /$$__/   /$$__  $$| $$  | $$
  /$$$$$$$$| $$|  $$$$$$$ /$$$$$$$$|  $$$$$$$|  $$$$$$$
 |________/|__/ \____  $$|________/ \_______/ \____  $$
                /$$  \ $$                     /$$  \ $$
               |  $$$$$$/                    |  $$$$$$/
                \______/                      \______/

        ⚡ A blazing fast HTTP client built with Go ⚡`

const body = `  DESCRIPTION
    A fast and elegant HTTP client for API testing and development.

  USAGE
    zz [FLAGS] [METHOD] <URL> ['<json body>']
    zz <COMMAND>

  COMMANDS
    ⚉ help               Show this help message
    ⚉ version, v         Show the version
    ⚉ run [file]         Read a .zap collection file (prints its tokens; requests are not sent yet)
    ⚉ (no args)          Interactive TUI (not built yet)

  METHODS
    get, post, put, patch, delete
    Short forms: g, p (or po), pu, pa, d. Case insensitive. Defaults to GET if omitted.

  FLAGS (go before the URL)
    ⚉ -H "Key: Value"    Add a request header (repeatable)
    ⚉ -v                 Show the URL, time and status (same as -u -t -s)
    ⚉ -u                 Show the URL
    ⚉ -t                 Show the time taken
    ⚉ -s                 Show the status
    ⚉ -n                 Print the body as received (no JSON pretty-printing)
    ⚉ -h                 Use http:// for URLs typed without a scheme

  EXAMPLES
    Basic requests:
      zz httpbin.org/get
      zz -v get https://api.github.com/users/octocat

    With headers (flags go first):
      zz -H "Authorization: Bearer token" /api

    POST with a JSON body:
      zz post /msg '{"message":"Hello!"}'
      zz -H "X-Api-Key: abc" post /users '{"name":"Alice"}'

    Local server over plain http:
      zz /api                        # http://127.0.0.1:8000/api
      zz -h localhost:3000/health    # http://localhost:3000/health

    Save the raw body:
      zz -n /file > out.json

  NOTES
    ⚉ /path goes to the host and port in ~/.config/zz/setting.json (default 127.0.0.1:8000)
    ⚉ URLs typed without a scheme use https://
    ⚉ A body must be valid JSON; Content-Type: application/json is added unless you pass your own -H
    ⚉ Flags go before the URL: zz -v /users, not zz /users -v
    ⚉ The URL, time and status go to stderr and the body to stdout, so output can be piped

  MORE INFO
    Repository: https://github.com/kinxyo/zz-go
    Issues:     https://github.com/kinxyo/zz-go/issues

`
