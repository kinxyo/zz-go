# zz ⚡

## Port list: Zig `zigzag` (v0.8, Jan 2026) vs this Go version

Based on the Zig code (`cli.zig`, `http.zig`, `file.zig`), not its `help.zig`, which is ahead of the code.

### In Zig, missing in Go

| # | Area | Feature | Zig behavior | Go today |
|---|------|---------|--------------|----------|
| 1 | Request | Custom headers | `-h "Key: Value"` (repeatable), `-hj '{"K":"V"}'` | None (TODO in `cli/helpers.go`) |
| 2 | Request | Default `User-Agent` | Always sends `zigzag` | Go's default `Go-http-client/1.1` |
| 3 | Request | Flags anywhere in args | `zz /x -v` works (manual arg loop) | `flag` stops at first non-flag arg; `zz /x -v` errors |
| 4 | Request | Free-form body | Any string is sent as-is | Must start with `{` and end with `}` |
| 5 | Request | No-body check | POST/PUT/PATCH without body fails with `NoPayload` before connecting | Sends an empty body |
| 6 | Request | Port flag | `-p <port>` for `/path` URLs | Hard-coded `127.0.0.1:8000` (`network/helper.go`) |
| 7 | Request | Header validation | `InvalidHeaderFormat` on malformed headers | N/A (no headers) |
| 8 | Collections | `run` executes requests | JSON collection `{name, baseUrl, apis[{method, path, headers, body}]}`, sent one by one | Lexes the `.zap` file and prints tokens only |
| 9 | Collections | Multiple files | `zz run a.json b.json` | Only `args[1]` |
| 10 | Collections | Default file | `test.json` in cwd | Hard-coded `example/test.zap` (breaks outside the repo) |
| 11 | Collections | Per-request headers/body | Supported in each `apis[]` entry | Unimplemented tokens only |
| 12 | UX | `help` command | `zz help` with logo and sections | `flag` usage text only; `-h` is taken by force-http |
| 13 | UX | Unknown flag | Warns and continues | `flag` prints usage and exits |
| 14 | UX | Method aliases | also `pat`, `del`, `de` | has `po` instead |
| 15 | UX | Buffered stdout/stderr | `io.zig` wrapper | Unbuffered (not important at this size) |

### In Go, missing in Zig

| Feature | Notes |
|---------|-------|
| Pretty-printed JSON | `-n` disables it. Zig prints the raw body. |
| Request timing | `-t`. Note: stops before the body is read. |
| Status line | `-s` (and `-u` for URL). Colors by class: 2xx green, 3xx cyan, 4xx yellow, 5xx red. Zig colors only a few codes. |
| `zz version`, CI workflow | CI pins Go 1.21 while `go.mod` says 1.25. |

### Missing in both

| Feature | Notes |
|---------|-------|
| Tests | None in either. |
| Concurrent requests | Zig: `// TODO`. Go: `-q` is parsed but unused. |
| Timeouts | Go: `&http.Client{}` per request, no timeout. |
| TUI | Stub in both. |
