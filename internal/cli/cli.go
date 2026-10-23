package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/kinxyo/zz-go/internal/config"
	"github.com/kinxyo/zz-go/internal/network"
	"github.com/kinxyo/zz-go/internal/terminal"
)

func Run(args []string, flags *terminal.Flags, settings config.Settings, userAgent string) {

	r, err := extract(args, flags, settings, userAgent)
	if err != nil {
		terminal.Fatal("%s\n", err)
	}

	res, err := network.CURL(r.method, r.url, r.headers, r.payload)
	if err != nil {
		terminal.Fatal("%s\n", err)
	}

	res.Print(printOpts(flags, r.method))

	if failed(res.StatusCode, *flags.Fail) {
		terminal.ExitWith(exitHTTPError)
	}
}

// exitHTTPError is what -f exits with for a 4xx or 5xx response, the same as curl.
// A request that could not be made at all exits with 1.
const exitHTTPError = 22

// failed reports whether -f should turn this response into a failure.
func failed(status int, fail bool) bool {
	return fail && status >= 400
}

// printOpts decides what to show. -v turns on all the metadata. HEAD and OPTIONS
// replies have nothing but a status and headers, so those are always shown.
func printOpts(flags *terminal.Flags, method string) network.PrintOpts {
	v := *flags.Verbose
	bodyless := method == "HEAD" || method == "OPTIONS"

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
	payload string
	headers http.Header
	// query   string
	// params  string
}

// extract builds a request from `[method] <url> ['{...}']` and the flags.
func extract(args []string, flags *terminal.Flags, settings config.Settings, userAgent string) (request, error) {

	r := request{method: "GET"}

	// TODO: query := parseHeaders(args)
	// TODO: params := parseHeaders(args)

	if m, ok := network.MatchMethod(args[0]); ok {
		// zz get <url>
		r.method, args = m, args[1:]
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
		// zz <url> '{...}'
		if !json.Valid([]byte(args[1])) {
			// two URLs in a row: the first one was meant to be a method
			if looksLikeURL(args[1]) {
				return r, fmt.Errorf("unknown method %q (use get, post, put, patch, delete, head or options)", args[0])
			}
			if strings.HasPrefix(args[1], "-") {
				return r, fmt.Errorf("%q looks like a flag; flags go before the arguments", args[1])
			}
			return r, fmt.Errorf("payload %q is not valid JSON", args[1])
		}
		r.payload = args[1]
	}

	h, err := buildHeaders(r.payload, *flags.Headers, userAgent)
	if err != nil {
		return r, err
	}
	r.headers = h

	return r, nil
}

// buildHeaders layers the headers, lowest priority first: defaults, then the -H flags.
// Set means a later header replaces an earlier one with the same name.
func buildHeaders(payload string, raw []string, userAgent string) (http.Header, error) {
	h := http.Header{}

	if userAgent != "" {
		h.Set("User-Agent", userAgent)
	}

	if payload != "" {
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
