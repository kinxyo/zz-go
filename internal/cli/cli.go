package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kinxyo/zz-go/internal/config"
	"github.com/kinxyo/zz-go/internal/network"
	"github.com/kinxyo/zz-go/internal/terminal"
)

// Env is what Run needs besides the arguments and the flags.
type Env struct {
	Settings  config.Settings
	UserAgent string // sent unless the user sets their own with -H
}

func Run(args []string, flags *terminal.Flags, env Env) {

	r, err := extract(args, flags, env.Settings)
	if err != nil {
		terminal.FatalWith(exitUsage, "%s\n", err)
	}

	if r.hasBody && r.bodyArg == "-" && terminal.StdinIsTerminal() {
		terminal.Log("reading the body from stdin (end it with Ctrl-D)")
	}

	if err := r.resolve(flags, env.UserAgent, os.Stdin); err != nil {
		terminal.FatalWith(exitUsage, "%s\n", err)
	}

	res, err := network.CURL(r.method, r.url, r.headers, r.payload)
	if err != nil {
		terminal.FatalWith(exitCode(err), "%s\n", err)
	}

	shown := res.Print(printOpts(flags, r.method))

	if failed(res.StatusCode, *flags.Fail) {
		terminal.ExitWith(exitHTTPError)
	}

	if !shown {
		terminal.ExitWith(exitOutputRefused)
	}
}

// failed reports whether -f should turn this response into a failure.
func failed(status int, fail bool) bool {
	return fail && status >= 400
}

// printOpts decides what to show. -v turns on all the metadata. HEAD and OPTIONS
// replies have nothing but a status and headers, so those are always shown.
func printOpts(flags *terminal.Flags, method string) network.PrintOpts {
	v := *flags.Verbose
	bodyless := method == http.MethodHead || method == http.MethodOptions

	return network.PrintOpts{
		URL:     v || *flags.PrintURL,
		Time:    v || *flags.PrintTime,
		Status:  v || bodyless || *flags.PrintStatus,
		Headers: v || bodyless || *flags.PrintHeaders,
		Pretty:  *flags.Pretty,
	}
}

type request struct {
	method  string
	url     string
	hasBody bool
	bodyArg string // the body as typed: inline, @file or -
	payload string // the body itself, once resolve has read it
	headers http.Header
	// query   string
	// params  string
}

// extract builds a request from `[method] <url> [body]`. It only reads the arguments;
// resolve reads a body from a file or stdin and builds the headers.
func extract(args []string, flags *terminal.Flags, settings config.Settings) (request, error) {

	r := request{method: http.MethodGet}
	methodGiven := false

	// TODO: query := parseHeaders(args)
	// TODO: params := parseHeaders(args)

	if m, ok := network.MatchMethod(args[0]); ok {
		// zz get <url>
		r.method, args = m, args[1:]
		methodGiven = true
	}

	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return r, errors.New("missing URL")
	}

	if strings.HasPrefix(args[0], "-") {
		return r, fmt.Errorf("%q looks like a flag; flags go before the arguments", args[0])
	}

	// zz <url>
	r.url = network.MakeURL(args[0], settings.BaseURL(), *flags.ForceHttp)

	if len(args) > 1 {
		// zz <url> '{...}' | @file | -
		arg := args[1]
		r.hasBody, r.bodyArg = true, arg

		if arg != "-" && !strings.HasPrefix(arg, "@") && !json.Valid([]byte(arg)) {
			// two URLs in a row: the first one was meant to be a method
			if looksLikeURL(arg) {
				return r, fmt.Errorf("unknown method %q (use get, post, put, patch, delete, head or options)", args[0])
			}
			if strings.HasPrefix(arg, "-") {
				return r, fmt.Errorf("%q looks like a flag; flags go before the arguments", arg)
			}
		}
	}

	// a body means there is something to send: POST, unless the user chose a method
	if r.hasBody && !methodGiven {
		r.method = http.MethodPost
	}

	return r, nil
}

// resolve reads the body (stdin is used for "-") and builds the headers. A body that isn't JSON
// is only accepted when the user set their own Content-Type, and not a JSON one.
func (r *request) resolve(flags *terminal.Flags, userAgent string, stdin io.Reader) error {
	if r.hasBody {
		body, err := resolveBody(r.bodyArg, stdin)
		if err != nil {
			return err
		}
		r.payload = body
	}

	h, err := buildHeaders(r.payload, *flags.RequestHeaders, userAgent)
	if err != nil {
		return err
	}
	r.headers = h

	if r.payload != "" && !json.Valid([]byte(r.payload)) {
		switch contentType := h.Get("Content-Type"); {
		case contentType == "":
			return fmt.Errorf("%s is not valid JSON; to send it as is, set a Content-Type with -H", describeBody(r.bodyArg))
		case isJSONType(contentType):
			return fmt.Errorf("%s is not valid JSON, but the Content-Type is %s", describeBody(r.bodyArg), contentType)
		}
	}

	return nil
}

// isJSONType reports whether a Content-Type names JSON: application/json, text/json or anything
// ending in +json. Types like application/x-ndjson are not JSON documents, so they don't count.
func isJSONType(contentType string) bool {
	media, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		media = strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	}

	return media == "application/json" || media == "text/json" || strings.HasSuffix(media, "+json")
}

// resolveBody returns the body for an argument that is inline, "@path" (a file) or "-" (stdin).
func resolveBody(arg string, stdin io.Reader) (string, error) {
	body := arg

	switch {
	case arg == "-":
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		body = string(b)

	case strings.HasPrefix(arg, "@"):
		b, err := os.ReadFile(expandHome(arg[1:]))
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("reading the body: %w (to send text that starts with @, pipe it in with -)", err)
		}
		if err != nil {
			return "", fmt.Errorf("reading the body: %w", err)
		}
		body = string(b)
	}

	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("%s is empty", describeBody(arg))
	}

	return body, nil
}

// expandHome turns a leading ~/ into the home directory. The shell does that for a word that
// starts with ~, but not for the part after an @.
func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	return filepath.Join(home, path[1:])
}

// describeBody names where a body came from, for error messages.
func describeBody(arg string) string {
	switch {
	case arg == "-":
		return "stdin"
	case strings.HasPrefix(arg, "@"):
		return fmt.Sprintf("file %q", arg[1:])
	default:
		return fmt.Sprintf("payload %q", arg)
	}
}

// buildHeaders layers the headers, lowest priority first: defaults, then the -H flags.
// Set means a later header replaces an earlier one with the same name.
func buildHeaders(payload string, raw []string, userAgent string) (http.Header, error) {
	h := http.Header{}

	if userAgent != "" {
		h.Set("User-Agent", userAgent)
	}

	if payload != "" && json.Valid([]byte(payload)) {
		h.Set("Content-Type", "application/json")
	}

	for _, s := range raw {
		// Cut at the first colon only; values may contain colons.
		k, v, ok := strings.Cut(s, ":")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if !ok || k == "" {
			return nil, fmt.Errorf("header %q: want \"Key: Value\"", s)
		}
		h.Set(k, v)
	}

	return h, nil
}

func looksLikeURL(s string) bool {
	return strings.HasPrefix(s, "/") || strings.Contains(s, "://")
}
