# zigzag

A command-line HTTP client built for firing quick requests at an API. The binary is called `zz`.

```sh
zz /health                                  # GET http://127.0.0.1:8000/health
zz post /users '{"name":"Alice"}'           # POST with a JSON body, Content-Type set for you
zz -H "Authorization: Bearer $TOKEN" /me    # add a header
zz -v httpbin.org/json                      # show the URL, time, status and headers as well
```

## Why

`curl` can do all of this, but every request costs typing:

```sh
curl -s -H 'Content-Type: application/json' -d '{"name":"Alice"}' http://127.0.0.1:8000/users | jq
zz -p post /users '{"name":"Alice"}'
```

- **Short.** Methods have short forms (`p` for POST), the method is optional (GET), and a path
  starting with `/` goes to your local server, so there's no host or port to type.
- **Metadata stays out of the way.** The URL, time, status and response headers go to stderr and
  the body to stdout, so `zz /users | jq` and `zz /file > out.json` stay clean. `-p` pretty-prints
  a JSON body when you want to read it.
- **JSON without the quotes.** Type the body the way you'd write it by hand; zz quotes the keys
  and the bare words for you. Valid JSON is never changed, and `-r` sends the body as typed.

  | Typed | Sent |
  |-------|------|
  | `{msg: hello world, n: 3, ok: true, x: null}` | `{"msg": "hello world", "n": 3, "ok": true, "x": null}` |
  | `{url: http://x.io/a?b=1, tags: [a, b, 2]}` | `{"url": "http://x.io/a?b=1", "tags": ["a", "b", 2]}` |
  | `{user: {name: alice, age: 30}}` | `{"user": {"name": "alice", "age": 30}}` |
  | `{say: he said "hi"}` | `{"say": "he said \"hi\""}` |
  | `{a: -1.5e3, b: 01}` | `{"a": -1.5e3, "b": "01"}`: `01` isn't a valid JSON number, so it stays a string, which keeps ZIP codes intact |
- **Plain errors.** Connection refused, unknown host and a wrong `https://` say what happened in
  one line instead of a Go error chain.
- **One binary.** No runtime, no config needed to start.

## Install

Download the binary for your system from the
[latest release](https://github.com/kinxyo/zz-go/releases/latest):

```sh
curl -L -o zz https://github.com/kinxyo/zz-go/releases/latest/download/zz-linux-amd64
chmod +x zz
mv zz ~/bin/
```

Builds are published for `linux`, `darwin` (macOS) and `windows` (`zz-windows-amd64.exe`), on
`amd64`, plus `arm64` for linux and macOS. Each release has a `SHA256SUMS` file.

With Go installed:

```sh
go install github.com/kinxyo/zz-go/cmd/zz@latest
```

From source (installs to `~/bin/zz`):

```sh
git clone https://github.com/kinxyo/zz-go
cd zz-go
make prod
```

## Use

```
zz [flags] [method] <url> [body]
```

**Flags go anywhere.** `zz -v /users` and `zz /users -v` are the same request.

### URLs

| You type                 | zz requests                               |
|--------------------------|-------------------------------------------|
| `/users`                 | `http://127.0.0.1:8000/users` (see [Config](#config)) |
| `httpbin.org/json`       | `https://httpbin.org/json`                |
| `localhost:3000/x`       | `http://localhost:3000/x` (`localhost`, `127.x.x.x` and `::1` are plain http) |
| `-d 192.168.1.5:8000/x`  | `http://192.168.1.5:8000/x`               |
| `https://api.example.com/x` | used as typed                          |

A query string is part of the URL: `zz '/search?q=go&page=2'`. Quote it, or the shell treats `&` as
"run in the background".

### Methods

`get`, `post`, `put`, `patch`, `delete`, `head` and `options`, in any case, with short forms: `g`,
`p` (or `po`), `pu`, `pa`, `d`, `he`, `opt` (or `o`). With no method the request is a GET, even
when you give a body, so type the method to send anything else: `zz p /users '{name: Alice}'`. A
first word that isn't a method is taken as the URL, so a typo like `zz gte /x` asks for
`https://gte`.

`head` and `options` replies have no body worth reading, so zz always shows their status and
headers (as if you had passed `-s -i`).

### Body

The argument after the URL is the body. Quote it in single quotes, so the shell passes it as one
argument and leaves `{`, `"` and `$` alone.

zz looks at the body to set the `Content-Type`:

| Body | Content-Type |
|------|--------------|
| `'{"a":1}'`, `'[1,2]'` | `application/json` |
| `'{a: 1}'` | `application/json`, after quoting it into `{"a": 1}` (see [Why](#why)) |
| one JSON value per line | `application/x-ndjson` |
| starts with `<` | `application/xml` (not checked) |
| any other text, like `'hello'` or `'a=1&b=2'` | `text/plain; charset=utf-8` |
| a file that isn't text (`@logo.png`) | `application/octet-stream` |
| no body | none |

Valid JSON is sent byte for byte, spacing included; only loose JSON is rewritten. A body that
starts with `{` or `[` but can't be made into JSON is an error that says what's wrong, so a typo
doesn't reach the server.

Your own `-H "Content-Type: ..."` always replaces the detected one, and `-r` turns detection off:
the body goes out exactly as typed, with no Content-Type unless you set one.

```sh
zz -H "Content-Type: application/x-www-form-urlencoded" post /login 'user=alice&pass=secret'
zz -r post /raw '{not: json}'
```

For a body that's too long to type, put it in a file and pass `@` and its path:

```sh
zz post /users @body.json
zz post /users @~/bodies/alice.json     # ~/ works after the @ too
```

The file is sent exactly as it is, including its final newline (like `curl --data-binary`;
`curl -d @file` strips newlines instead), and goes through the same Content-Type detection and
JSON fix-ups as a typed body. `-r` still reads the file but sends it untouched. An empty file is
an error. Because of `@`, a typed body can't start with `@`.

### Flags

| Flag | |
|------|-|
| `-H "Key: Value"` | Add a request header. Repeat it for more. A later header with the same name replaces an earlier one. |
| `-v` | Show the URL, time, status and response headers (the same as `-u -t -s -i`), after a first line saying which [project entry](#projects) of the config is in use. |
| `-u` | Show the URL. |
| `-t` | Show the time the request took. |
| `-s` | Show the status, colored by class (2xx green, 3xx cyan, 4xx yellow, 5xx red). |
| `-i` | Show the response headers. |
| `-p` | Pretty-print a JSON body. Without it the body is printed exactly as received. |
| `-f` | Exit with status 22 if the response is a 4xx or 5xx. Unlike `curl --fail`, the body is still printed, so you can read the error. |
| `-d` | Use `http://` for a URL typed without a scheme. Local hosts already get it. |
| `-r` | Send the body exactly as typed: no JSON fix-ups, no Content-Type detected. |
| `-m <seconds>` | Time limit for the whole request, in whole seconds; `0` means no limit. Overrides `timeout` in the [config](#config). |

Requests send `User-Agent: zz/<version>` unless you set your own: `userAgent` in the
[config](#config) for every request, or `-H` for one. An empty `-H "User-Agent:"` sends none.

Colors appear only when the output is a terminal; they are off when piped or redirected, or when
`NO_COLOR` is set.

### Other commands

```sh
zz help      # usage (also zz -h)
zz version   # the version
```

## Examples

Basic requests:

```sh
zz httpbin.org/get
zz -v get https://api.github.com/users/octocat
```

With headers:

```sh
zz -H "Authorization: Bearer $TOKEN" /api
zz -H "X-Api-Key: abc" -H "Accept: application/json" /items
```

POST with a JSON body:

```sh
zz post /msg '{"message":"Hello!"}'
zz -H "X-Api-Key: abc" post /users '{"name":"Alice"}'
zz post /users '{name: Alice, tags: [a, b]}'    # loose JSON, quoted for you
zz post /users @body.json                       # the body from a file
```

A local server over plain http:

```sh
zz /api                         # http://127.0.0.1:8000/api
zz localhost:3000/health        # http://localhost:3000/health
zz -d 192.168.1.5:8000/health   # http on another machine
```

Read a JSON response, or save it exactly as received:

```sh
zz -p /users
zz /file > out.json
```

## Config

The first request creates `~/.config/zz/setting.json` (on macOS
`~/Library/Application Support/zz/`, on Windows `%AppData%\zz\`):

```json
{
  "host": "127.0.0.1",
  "port": 8000,
  "timeout": 30,
  "userAgent": "",
  "projects": {}
}
```

`host` and `port` decide where `/path` URLs go. `timeout` is the time limit for a whole request
in whole seconds, including reading the body; `0` means no limit, and `-m` overrides it for one
request. `userAgent` replaces the `zz/<version>` User-Agent for every request; empty means
`zz/<version>`. A key you leave out keeps its default. If the file can't be read, or a value is
wrong (a port out of range, a negative or non-whole timeout, a key zz doesn't know), zz prints a
warning and uses the defaults.

### Projects

`projects` gives a directory its own settings, for requests made in it or in any directory
below it:

```json
{
  "port": 8000,
  "projects": {
    "~/work/shop-api": { "port": 3000 },
    "/srv/legacy": { "host": "10.0.0.5", "timeout": 5 }
  }
}
```

From `~/work/shop-api/handlers`, `zz /users` goes to port 3000; anywhere else, to 8000.

- An entry takes the same keys as the top level. A key it sets wins; a key it leaves out keeps
  the top-level value. `"timeout": 0` in an entry turns the limit off.
- A key is an absolute path; `~/` is your home directory. Symlinks are resolved on both sides.
- The nearest entry wins: an entry for `~/work/shop-api/admin` beats `~/work/shop-api` inside
  `admin`. A key only matches whole directory names, so `~/work/shop-api` doesn't cover
  `~/work/shop-api-v2`.
- A wrong entry (a bad value, a key zz doesn't know, a relative path) is skipped with a warning,
  and the top-level settings are used.
- Entries are tied to the path: rename or move a directory and its entry no longer matches.
  `-v` starts with the entry in use (`config: project ~/work/shop-api`, or `config: no project
  entry`).

## Output and errors

- The body goes to **stdout**, exactly as received. On a terminal zz ends the last line with a
  newline if the body doesn't, so your prompt starts on its own line; piped or redirected output
  is never changed. The URL, time, status, response headers and warnings go to **stderr**.
- A binary response (a body with a NUL byte, or that isn't text) is not printed to a terminal,
  because it would garble it. zz says so, shows the size and `Content-Type`, and exits with status
  23. Redirect it (`zz /logo.png > logo.png`) or pipe it (`zz /logo.png | cat`) to get the bytes.
- Response headers (`-i`) are listed alphabetically, with names in Go's standard form
  (`X-Request-Id`, whatever spelling the server used). zz can't show the server's original order
  or capitalization.
- A response with a 4xx or 5xx status is still a response: the body is printed and zz exits with
  status 0, unless you pass `-f`, which exits with 22.
- A request that fails always prints why, and exits with a code that says which failure it was
  (see below), so a script can tell them apart:

  ```sh
  until zz -f /health >/dev/null; do sleep 1; done     # wait for the server to come up
  ```
- A request times out after 30 seconds by default, counting the time spent reading the body. Set
  `timeout` in the config, or `-m <seconds>` for one request; `0` means no limit.

### Exit codes

Where curl has a code for the same situation, zz uses the same number.

| Code | Meaning | curl |
|------|---------|------|
| 0 | A response arrived (any status, unless `-f`) | 0 |
| 1 | Any other failure | |
| 2 | Bad arguments: missing URL, bad `-H`, bad body, unknown flag, malformed URL | |
| 6 | No such host | 6 |
| 7 | Connection refused | 7 |
| 22 | `-f` and the status was 4xx or 5xx | 22 |
| 23 | A binary body was not written to a terminal | 23 |
| 28 | The request timed out | 28 |
| 35 | `https://` to a server that speaks plain HTTP | 35 |
| 60 | The server's TLS certificate was not accepted | 60 |

## Not yet

- `zz run [file]` reads a `.zz` collection file and prints its tokens. It does not send the requests.
- Running `zz` with no arguments is a placeholder; there is no interactive mode yet.

## Development

```sh
make test    # go vet + go test -race
make run     # build and install to ~/bin
make reset   # delete setting.json, project entries included; the next request writes the defaults
make deps    # which package imports which
```

The tests have two layers. Unit tests sit next to the code they test. The end-to-end tests in
`cmd/zz/e2e_test.go` build the real `zz` once and run it against a local test server, each test
with its own config directory, checking stdout, stderr, the exit code and what the server
received. `go test -short ./...` skips them. A unit test in `internal/terminal` checks that every
flag the program has is listed in the help and in the flag table above, and the other way round.
Anything that needs a real terminal (colors, refusing binary output) is covered only by unit tests
of the helpers that decide it.

A release build stamps its version into the binary. To do the same by hand:

```sh
go build -ldflags "-X github.com/kinxyo/zz-go/internal/config.Version=1.2.3" -o zz ./cmd/zz
```

Every push to `main` runs the tests, builds the binaries above and publishes them as a release
tagged `v1.2.<run number>`.

## License

Apache 2.0, see [LICENSE](LICENSE).
