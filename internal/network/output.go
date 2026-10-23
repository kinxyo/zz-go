package network

import (
	"bytes"
	"encoding/json"
	"sort"

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
func (r *Result) Print(o PrintOpts) {
	if o.URL {
		terminal.LogFC(terminal.TextBold, "%s %s\n", r.Method, r.URL)
	}

	if o.Time {
		terminal.LogFC(terminal.TextDim, "Time: %v\n", r.Elapsed)
	}

	if o.Status {
		terminal.LogFC(statusColor(r.StatusCode), "%s\n", r.Status)
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

	// blank line between the metadata and the body
	if o.URL || o.Time || o.Status || o.Headers {
		terminal.Log()
	}

	if len(r.Body) == 0 {
		return
	}

	if o.Pretty {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, r.Body, "", "  "); err == nil {
			terminal.Print(pretty.String())
			return
		}
	}

	terminal.PrintF("%s", r.Body)
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
