package collection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kinxyo/zz-go/internal/network"
	"github.com/kinxyo/zz-go/internal/terminal"
)

type outcome int

const (
	passed outcome = iota
	failed
	skipped
)

// line is one request's result in the report.
type line struct {
	name, method, url string // the url as written in the file, so a secret in it isn't shown
	outcome           outcome
	note              string           // in place of the status: why it failed to send, or was skipped
	problems          []string         // listed under the line: the captures that found nothing
	sent              *network.Request // nil when it couldn't be built
	res               *network.Result  // nil when nothing came back
}

// report prints a line per request, in columns, and counts the outcomes.
type report struct {
	name, method, url int // column widths; name is 0 when no request has one
	verbose, pretty   bool
	counts            [3]int // by outcome
	spaced            bool   // the last thing printed ends with a blank line (-v details)
}

const maxURLWidth = 40 // a longer url pushes the rest of its line instead of every line

func (c *Collection) newReport(verbose, pretty bool) *report {
	rp := &report{verbose: verbose, pretty: pretty}

	widen := func(name string, r *Request) {
		method, _ := network.MatchMethod(r.Method)
		rp.name = max(rp.name, len([]rune(name)))
		rp.method = max(rp.method, len(method))
		rp.url = max(rp.url, min(len([]rune(r.URL)), maxURLWidth))
	}

	if c.Auth != nil && c.Auth.Login != nil {
		widen("login", c.Auth.Login)
	}
	for i := range c.Requests {
		widen(c.Requests[i].Name, &c.Requests[i])
	}

	return rp
}

// print writes l's line, its problems under it, and with -v what was sent and what came back.
func (rp *report) print(l line) {
	rp.counts[l.outcome]++
	rp.spaced = false

	mark, color := "✓", terminal.TextGreen
	switch l.outcome {
	case failed:
		mark, color = "✗", terminal.TextRed
	case skipped:
		mark, color = "-", terminal.TextDim
	}

	var b strings.Builder
	if rp.name > 0 {
		fmt.Fprintf(&b, "%-*s  ", rp.name, l.name)
	}
	fmt.Fprintf(&b, "%-*s %-*s  ", rp.method, l.method, rp.url, l.url)
	if l.res != nil && l.note == "" {
		fmt.Fprintf(&b, "%d %6s", l.res.StatusCode, millis(l.res.Elapsed))
	} else {
		b.WriteString(l.note)
	}

	terminal.PrintFC(color, "%s ", mark)
	if l.outcome == skipped {
		terminal.PrintFC(terminal.TextDim, "%s\n", b.String())
	} else {
		terminal.PrintF("%s\n", b.String())
	}

	for _, p := range l.problems {
		terminal.PrintFC(terminal.TextRed, "    %s\n", p)
	}

	if rp.verbose && l.sent != nil {
		rp.details(l)
	}
}

// details shows what -v adds under a line: the full URL sent, then the response's status line,
// headers and body.
func (rp *report) details(l line) {
	const indent = "    "

	terminal.PrintFC(terminal.TextDim, indent+"%s %s\n", l.sent.Method, l.sent.URL)

	if res := l.res; res != nil {
		terminal.PrintF(indent+"%s %s\n", res.Proto, res.Status)

		names := make([]string, 0, len(res.Header))
		for name := range res.Header {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			for _, v := range res.Header[name] {
				terminal.PrintFC(terminal.TextDim, indent+"%s: %s\n", name, v)
			}
		}

		switch {
		case len(res.Body) == 0:
		case res.IsBinary():
			terminal.PrintFC(terminal.TextDim, "\n"+indent+"binary body (%d bytes) not shown\n", len(res.Body))
		default:
			terminal.PrintF("\n%s\n", indentBody(res.Body, indent, rp.pretty))
		}
	}

	terminal.Print()
	rp.spaced = true
}

// indentBody puts indent before every line of body, indenting JSON first with -p.
func indentBody(body []byte, indent string, pretty bool) string {
	if pretty {
		var b bytes.Buffer
		if json.Indent(&b, body, indent, "  ") == nil {
			return indent + b.String()
		}
	}

	return indent + strings.ReplaceAll(strings.TrimRight(string(body), "\n"), "\n", "\n"+indent)
}

// summary ends a run that got past login.
func (rp *report) summary() {
	rp.space()
	terminal.PrintFC(terminal.TextBold, "%d passed, %d failed, %d skipped\n", rp.counts[passed], rp.counts[failed], rp.counts[skipped])
}

// stopped ends a run whose login failed.
func (rp *report) stopped(left int) {
	rp.space()
	if left == 1 {
		terminal.PrintFC(terminal.TextBold, "login failed, so the 1 request after it wasn't sent\n")
		return
	}
	terminal.PrintFC(terminal.TextBold, "login failed, so the %d requests after it weren't sent\n", left)
}

// space puts a blank line before the last line, unless there is one already.
func (rp *report) space() {
	if !rp.spaced {
		terminal.Print()
	}
}

func millis(d time.Duration) string {
	return fmt.Sprintf("%dms", d.Milliseconds())
}
