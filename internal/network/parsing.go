package network

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// MatchMethod returns the HTTP method for m (or an alias of it), and false if m isn't one.
func MatchMethod(m string) (string, bool) {
	m = strings.ToLower(m)
	m = strings.TrimSpace(m)

	switch m {
	case "get", "g":
		return http.MethodGet, true
	case "post", "p", "po":
		return http.MethodPost, true
	case "put", "pu":
		return http.MethodPut, true
	case "patch", "pa":
		return http.MethodPatch, true
	case "delete", "d":
		return http.MethodDelete, true
	case "head", "he":
		return http.MethodHead, true
	case "options", "opt", "o":
		return http.MethodOptions, true
	default:
		return "", false
	}
}

// MakeURL turns the URL the user typed into a full one, in place. base is where paths
// like "/users" go.
func (r *Request) MakeURL(base string, forceHTTP bool) {
	// Example:
	// 1. zz /api/users (local, from the config)
	// 2. zz httpbin.org/json (https)
	// 3. zz -d httpbin.org/json (http)
	// 4. zz localhost:3000/x (http: a server on this machine is plain http)
	// A URL that already has a scheme is left alone, even with -d.

	switch {
	case strings.HasPrefix(r.URL, "/"):
		r.URL = base + r.URL
	case strings.HasPrefix(r.URL, "http://") || strings.HasPrefix(r.URL, "https://"):
		// used as typed
	case forceHTTP, isLoopback(r.URL):
		r.URL = "http://" + r.URL
	default:
		r.URL = "https://" + r.URL
	}
}

// isLoopback reports whether a URL typed without a scheme points at this machine:
// localhost, 127.x.x.x or ::1, with or without a port.
func isLoopback(url string) bool {
	host := url
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]") // [::1] without a port

	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ParseBody returns the Content-Type of r.Body, or "" when it can't tell. r.Body only
// changes when loose JSON is fixed up.
// Only the types that can be told apart reliably are detected:
//   - JSON: the whole body parses as one JSON value. JSON typed loosely by hand, like
//     {content: hi}, is fixed up first (see relaxJSON); valid JSON is never changed.
//   - NDJSON: more than one line, and every line is a JSON value on its own.
//   - XML: the body starts with '<'. HTML starts with '<' too, but nobody sends
//     HTML to an API, so '<' is taken to mean XML. It isn't validated.
//
// A body that starts like JSON ({ or [) but is neither is an error that says what's wrong.
//
// Not detected (yet):
//   - form-urlencoded (a=1&b=2): only a guess, it clashes with plain text like "x=1".
//   - multipart: not typed by the user; the tool builds it from files, with its own boundary.
//   - file types for @file: by extension (mime.TypeByExtension), then by sniffing the
//     first bytes (http.DetectContentType).
func (r *Request) ParseBody() (string, error) {
	trimmed := strings.TrimSpace(r.Body)

	if trimmed == "" {
		return "", nil
	}

	switch trimmed[0] {
	case '<':
		return "application/xml", nil

	case '{', '[':
		var v any
		err := json.Unmarshal([]byte(trimmed), &v)
		if err == nil {
			return "application/json", nil
		}

		if fixed := relaxJSON(trimmed); json.Valid([]byte(fixed)) {
			r.Body = fixed
			return "application/json", nil
		}

		if isNDJSON(trimmed) {
			return "application/x-ndjson", nil
		}

		return "", fmt.Errorf("body is not valid JSON: %w", err)
	}

	return "", nil
}

// relaxJSON quotes the bare words in JSON typed by hand: {content: hi} becomes
// {"content":"hi"}. A key runs up to its ':', a value up to the next , } or ], so
// {msg: hello world, url: http://x.io} works too. Numbers, true, false and null are
// left alone, and quoted strings are copied as they are.
func relaxJSON(s string) string {
	var out strings.Builder
	var open []byte // the { and [ not closed yet
	key := false    // in an object, the next bare word is a key

	for i := 0; i < len(s); {
		c := s[i]

		switch c {
		case '"':
			j := i + 1
			for j < len(s) && s[j] != '"' {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			j = min(j+1, len(s))
			out.WriteString(s[i:j])
			i = j
			continue

		case '{', '[':
			open = append(open, c)
			key = c == '{'

		case '}', ']':
			if len(open) > 0 {
				open = open[:len(open)-1]
			}
			key = false

		case ',':
			key = len(open) > 0 && open[len(open)-1] == '{'

		case ':':
			key = false

		case ' ', '\t', '\n', '\r':

		default:
			stops := ",}]"
			if key {
				stops = ":,{}[]"
			}

			j := i
			for j < len(s) && !strings.ContainsRune(stops, rune(s[j])) {
				j++
			}

			word := strings.TrimSpace(s[i:j])
			if !key && json.Valid([]byte(word)) {
				out.WriteString(word) // a number, true, false or null
			} else {
				q, _ := json.Marshal(word)
				out.Write(q)
			}
			i = j
			continue
		}

		out.WriteByte(c)
		i++
	}

	return out.String()
}

// isNDJSON reports whether body has more than one line and every non-empty line is valid JSON.
func isNDJSON(body string) bool {
	lines := 0

	for line := range strings.Lines(body) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !json.Valid([]byte(line)) {
			return false
		}
		lines++
	}

	return lines > 1
}

// BuildHeaders layers the headers, lowest priority first: zz's defaults (the User-Agent and the
// Content-Type ParseBody detected), then the -H flags. Set replaces an earlier header with the
// same name, and names are matched without case, so -H "content-type: text/plain" wins over the
// detected one. An empty -H value, like -H "User-Agent:", sends that header with no value, which
// for the User-Agent means none is sent.
// r.Header is only set when every -H is well formed.
func (r *Request) BuildHeaders(contentType string, raw []string, userAgent string) error {
	h := http.Header{}

	// 1. Setting defaults
	if userAgent != "" {
		h.Set("User-Agent", userAgent)
	}

	if contentType != "" {
		h.Set("Content-Type", contentType)
	}

	// 2. User overrides
	for _, s := range raw {
		k, v, ok := strings.Cut(s, ":") // Cut at the first colon only; values may contain colons.
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if !ok || k == "" {
			return fmt.Errorf("header %q: want \"Key: Value\"", s)
		}
		h.Set(k, v)
	}

	r.Header = h
	return nil
}
