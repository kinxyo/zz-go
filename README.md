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
zz post /users '{"name":"Alice"}'
```

- **Short.** Methods have short forms (`p` for POST), the method is optional (GET), and a path
  starting with `/` goes to your local server, so there's no host or port to type.
- **Metadata stays out of the way.** The URL, time, status and response headers go to stderr and
  the body to stdout, so `zz /users | jq` and `zz /file > out.json` stay clean. `-p` pretty-prints
  a JSON body when you want to read it.
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
zz [flags] [method] <url> ['<json body>']
```

**Flags go before the URL.** `zz -v /users` works; `zz /users -v` is rejected.

### URLs

| You type                 | zz requests                               |
|--------------------------|-------------------------------------------|
| `/users`                 | `http://127.0.0.1:8000/users` (see [Config](#config)) |
| `httpbin.org/json`       | `https://httpbin.org/json`                |
| `-d localhost:3000/x`    | `http://localhost:3000/x`                 |
| `https://api.example.com/x` | used as typed                          |

A query string is part of the URL: `zz '/search?q=go&page=2'`. Quote it, or the shell treats `&` as
"run in the background".

### Methods

`get`, `post`, `put`, `patch`, `delete`, `head` and `options`, in any case, with short forms: `g`,
`p` (or `po`), `pu`, `pa`, `d`, `he`, `opt` (or `o`). With no method, the request is a GET.

`head` and `options` replies have no body worth reading, so zz always shows their status and
headers (as if you had passed `-s -i`).

### Body

The last argument is the body. It can be given three ways:

| You type | The body is |
|----------|-------------|
| `'{"a":1}'` | the text you typed |
| `@body.json` | the contents of that file |
| `-` | whatever is on stdin |

A JSON body gets `Content-Type: application/json` automatically. To avoid quoting JSON in the
shell at all, put it in a file, or use a heredoc (the quoted `'EOF'` means nothing inside is
treated specially, so no escaping):

```sh
zz post /users @body.json

zz post /users - <<'EOF'
{"name": "Alice", "tags": ["a", "b"]}
EOF
```

A body that isn't valid JSON is only sent if you set your own `Content-Type`; without one, zz
stops with an error, which catches typos like `zz post /users hello`. Your header also replaces the
default for JSON bodies:

```sh
zz -H "Content-Type: application/x-www-form-urlencoded" post /login 'user=alice&pass=secret'
zz -H "Content-Type: application/vnd.api+json" post /things '{"data":{}}'
```

An empty body (an empty file, empty stdin, or `""`) is an error.

### Flags

| Flag | |
|------|-|
| `-H "Key: Value"` | Add a request header. Repeat it for more. A later header with the same name replaces an earlier one. |
| `-v` | Show the URL, time, status and response headers (the same as `-u -t -s -i`). |
| `-u` | Show the URL. |
| `-t` | Show the time the request took. |
| `-s` | Show the status, colored by class (2xx green, 3xx cyan, 4xx yellow, 5xx red). |
| `-i` | Show the response headers. |
| `-p` | Pretty-print a JSON body. Without it the body is printed exactly as received. |
| `-f` | Exit with status 22 if the response is a 4xx or 5xx, like `curl --fail`. The body is still printed. |
| `-d` | Use `http://` for a URL typed without a scheme. |

Requests send `User-Agent: zz/<version>` unless you set your own with `-H`.

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

With headers (flags go first):

```sh
zz -H "Authorization: Bearer $TOKEN" /api
zz -H "X-Api-Key: abc" -H "Accept: application/json" /items
```

POST with a JSON body:

```sh
zz post /msg '{"message":"Hello!"}'
zz -H "X-Api-Key: abc" post /users '{"name":"Alice"}'
zz post /users @body.json
```

A local server over plain http:

```sh
zz /api                         # http://127.0.0.1:8000/api
zz -d localhost:3000/health     # http://localhost:3000/health
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
  "port": 8000
}
```

This decides where `/path` URLs go. A key you leave out keeps its default. If the file can't be
read, zz prints a warning and uses the defaults.

## Output and errors

- The body goes to **stdout**, exactly as received (no newline is added). The URL, time, status,
  response headers and warnings go to **stderr**.
- A response with a 4xx or 5xx status is still a response: the body is printed and zz exits with
  status 0, unless you pass `-f`, which exits with 22. A request that fails (connection refused,
  unknown host, bad TLS certificate, a timeout, a malformed argument) exits with status 1, so a
  script can tell the two apart:

  ```sh
  until zz -f /health; do sleep 1; done     # wait for the server to come up
  ```
- A request times out after 30 seconds, counting the time spent reading the body.

## Not yet

- `zz run [file]` reads a `.zz` collection file and prints its tokens. It does not send the requests.
- Running `zz` with no arguments is a placeholder; there is no interactive mode yet.
- Flags after the URL aren't supported.

## Development

```sh
make test    # go vet + go test -race
make run     # build and install to ~/bin
make deps    # which package imports which
```

A release build stamps its version into the binary. To do the same by hand:

```sh
go build -ldflags "-X main.version=1.2.3" -o zz ./cmd/zz
```

Every push to `main` runs the tests, builds the binaries above and publishes them as a release
tagged `v1.0.<run number>`.

## License

Apache 2.0, see [LICENSE](LICENSE).
