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
    zz [METHOD] [URL] [OPTIONS]
    zz [COMMAND]

  COMMANDS
    ⚉ (no args)          Launch interactive TUI mode
    ⚉ run <file.json>    Execute API collection from file
    ⚉ help, -h, --help   Show this help message

  METHODS
    get, post, put, patch, delete, head, options
    (Case insensitive. Defaults to GET if omitted)

  OPTIONS
    ⚉ -v, --verbose      Show request/response details
    ⚉ -d, --dev          Development mode (use localhost)
    ⚉ -h <json>          Set request headers as JSON object
    ⚉ -b <json>          Set request body as JSON object
    ⚉ -p <port>          Specify port (default: 8000)

  EXAMPLES
    Basic requests:
      zz httpbin.org/get
      zz get https://api.github.com/users/octocat -v

    With headers:
      zz get /api -h '{"Authorization":"Bearer token"}' -v

    POST with body:
      zz post /msg '{"message":"Hello!"}' -v
      zz post /users -b '{"name":"Alice"}' -h '{"Content-Type":"application/json"}'

    Development mode:
      zz /api -d -v              # Hits http://localhost:8000/api
      zz post /data '{}' -d -p 3000

    Run collection:
      zz run tests.json

  NOTES
    ⚉ URLs without scheme default to http://
    ⚉ Relative paths in -d mode use http://localhost:<port>
    ⚉ JSON strings must be properly quoted in shell

  MORE INFO
    Repository: https://github.com/kinxyo/zigzap
    Issues:     https://github.com/kinxyo/zigzap/issues

`
