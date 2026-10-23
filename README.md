# zigzag

A command-line HTTP client built for firing quick requests at an API. The binary is called `zz`.

```sh
zz /health                                  # GET http://127.0.0.1:8000/health
zz post /users '{"name":"Alice"}'           # POST with a JSON body, Content-Type set for you
zz -H "Authorization: Bearer $TOKEN" /me    # add a header
zz -v httpbin.org/json                      # show the URL, time and status as well
```

## Why

`curl` can do all of this, but every request costs typing:

```sh
curl -s -H 'Content-Type: application/json' -d '{"name":"Alice"}' http://127.0.0.1:8000/users | jq
zz post /users '{"name":"Alice"}'
```

- **Short.** Methods have short forms (`p` for POST), the method is optional (GET), and a path
  starting with `/` goes to your local server, so there's no host or port to type.
- **Readable by default.** JSON responses are pretty-printed. The URL, time and status go to
  stderr and the body to stdout, so `zz /users | jq` stays clean.
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
| `-h localhost:3000/x`    | `http://localhost:3000/x`                 |
| `https://api.example.com/x` | used as typed                          |

A query string is part of the URL: `zz '/search?q=go&page=2'`. Quote it, or the shell treats `&` as
"run in the background".

### Methods

`get`, `post`, `put`, `patch`, `delete`, in any case, with short forms: `g`, `p` (or `po`), `pu`,
`pa`, `d`. With no method, the request is a GET.

### Body

The last argument is the body and must be valid JSON. `Content-Type: application/json` is added
whenever there is a body. To send something else, pass your own header, which takes priority:

```sh
zz -H "Content-Type: application/vnd.api+json" post /things '{"data":{}}'
```

### Flags

| Flag | |
|------|-|
| `-H "Key: Value"` | Add a request header. Repeat it for more. A later header with the same name replaces an earlier one. |
| `-v` | Show the URL, time and status (the same as `-u -t -s`). |
| `-u` | Show the URL. |
| `-t` | Show the time the request took. |
| `-s` | Show the status, colored by class (2xx green, 3xx cyan, 4xx yellow, 5xx red). |
| `-n` | Print the body exactly as received, with no pretty-printing. |
| `-h` | Use `http://` for a URL typed without a scheme. |

Colors appear only when the output is a terminal; they are off when piped or redirected, or when
`NO_COLOR` is set.

### Other commands

```sh
zz help      # usage
zz version   # the version
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

- The body goes to **stdout**. The URL, time and status (and warnings) go to **stderr**.
- A response with a 4xx or 5xx status is still a response: the body is printed and zz exits with
  status 0. Only a failed request (connection refused, unknown host, bad TLS certificate, a
  malformed argument) exits with status 1.

## Not yet

- `zz run [file]` reads a `.zz` collection file and prints its tokens. It does not send the requests.
- Running `zz` with no arguments is a placeholder; there is no interactive mode yet.
- There is no request timeout.
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
