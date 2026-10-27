package network

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/kinxyo/zz-go/internal/terminal"
)

// PrintOpts picks what Print shows. The body is always shown.
type PrintOpts struct {
	URL     bool
	Time    bool
	Status  bool
	Headers bool // the response headers
	Pretty  bool // indent a JSON body; otherwise the body is written exactly as received
}

// Print writes the metadata to stderr and the body to stdout, so the body can be piped.
// It returns false when it held the body back: a binary body is not written to a terminal.
func (r *Result) Print(o PrintOpts) bool {
	if o.URL {
		terminal.LogFC(terminal.TextBold, "%s %s\n", r.Method, r.URL)
	}

	if o.Time {
		terminal.LogFC(terminal.TextDim, "Time: %v\n", r.Elapsed)
	}

	if o.Status {
		terminal.LogFC(statusColor(r.StatusCode), "%s %s\n", r.Proto, r.Status)
	}

	if o.Headers {
		names := make([]string, 0, len(r.Header))
		for name := range r.Header {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			for _, value := range r.Header[name] {
				terminal.LogFC(terminal.TextDim, "%s: %s\n", name, value)
			}
		}
	}

	if len(r.Body) == 0 {
		return true
	}

	if terminal.StdoutIsTerminal() && looksBinary(r.Body, r.Header.Get("Content-Type")) {
		contentType := r.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "no Content-Type"
		}

		terminal.LogFC(terminal.TextYellow, "binary response (%d bytes, %s) not shown on a terminal: redirect it to a file (zz ... > out), or pipe it through cat to see it anyway\n", len(r.Body), contentType)
		return false
	}

	// blank line between the metadata and the body
	if o.URL || o.Time || o.Status || o.Headers {
		terminal.Log()
	}

	if o.Pretty {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, r.Body, "", "  "); err == nil {
			terminal.Print(pretty.String())
			return true
		}
	}

	terminal.PrintF("%s", r.Body)

	// piped or redirected output stays exactly as received; on a terminal, end the line
	// so the prompt doesn't start in the middle of it
	if r.Body[len(r.Body)-1] != '\n' && terminal.StdoutIsTerminal() {
		terminal.Print()
	}

	return true
}

// IsBinary reports whether the body would mess up a terminal (see looksBinary).
func (r *Result) IsBinary() bool {
	return looksBinary(r.Body, r.Header.Get("Content-Type"))
}

// looksBinary reports whether a body would mess up a terminal: it has a NUL byte, or it isn't valid
// UTF-8 and the server didn't say it is text (a Latin-1 page sent as text/html is still text).
func looksBinary(body []byte, contentType string) bool {
	if bytes.IndexByte(body, 0) >= 0 {
		return true
	}

	if utf8.Valid(body) {
		return false
	}

	return !strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "text/")
}

func statusColor(code int) terminal.TextFmt {
	switch code / 100 {
	case 2:
		return terminal.TextGreen
	case 3:
		return terminal.TextCyan
	case 4:
		return terminal.TextYellow
	case 5:
		return terminal.TextRed
	default:
		return terminal.Reset
	}
}
