package cli

import (
	"strings"
	"testing"

	"github.com/kinxyo/zz-go/internal/config"
	"github.com/kinxyo/zz-go/internal/network"
	"github.com/kinxyo/zz-go/internal/terminal"
)

// extract runs extractIntoRequest and turns a panic into a test failure, so one bad case
// doesn't stop the rest of the package's tests.
func extract(t *testing.T, args []string) (r network.Request, err error) {
	t.Helper()

	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("extractIntoRequest(%q) panicked: %v", args, p)
		}
	}()

	return extractIntoRequest(args, &terminal.Flags{}, config.Settings{})
}

// TestExtractIntoRequest checks that each argument lands in its slot as typed: the URL is
// not made full and the body is not parsed yet.
func TestExtractIntoRequest(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		method string
		url    string
		body   string
	}{
		{"bare path is a GET", []string{"/x"}, "GET", "/x", ""},
		{"bare host", []string{"httpbin.org/json"}, "GET", "httpbin.org/json", ""},
		{"method and url", []string{"get", "/x"}, "GET", "/x", ""},
		{"method is case insensitive", []string{"POST", "/x"}, "POST", "/x", ""},
		{"alias with body", []string{"p", "/x", `{"a":1}`}, "POST", "/x", `{"a":1}`},
		{"body is kept as typed", []string{"p", "/x", `{a: 1}`}, "POST", "/x", `{a: 1}`},
		{"no method is a GET, even with a body", []string{"/x", `{"a":1}`}, "GET", "/x", `{"a":1}`},
		{"get with a body stays a GET", []string{"get", "/x", `{"a":1}`}, "GET", "/x", `{"a":1}`},
		{"put", []string{"put", "/x", `{"a":1}`}, "PUT", "/x", `{"a":1}`},
		{"head", []string{"he", "/x"}, "HEAD", "/x", ""},
		{"options", []string{"o", "/x"}, "OPTIONS", "/x", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := extract(t, c.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Method != c.method || got.URL != c.url || got.Body != c.body {
				t.Errorf("got method=%q url=%q body=%q\nwant method=%q url=%q body=%q",
					got.Method, got.URL, got.Body, c.method, c.url, c.body)
			}
		})
	}
}

func TestExtractIntoRequestMissingURL(t *testing.T) {
	for _, args := range [][]string{{"get"}, {"get", ""}, {"post", "  "}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, err := extract(t, args); err == nil {
				t.Error("want an error for a missing URL")
			}
		})
	}
}

// meta is the set of output flags, with names, so a test can't mix them up.
type meta struct{ v, u, t, s, i, p bool }

func (m meta) flags() *terminal.Flags {
	return &terminal.Flags{
		Verbose: &m.v, PrintURL: &m.u, PrintTime: &m.t,
		PrintStatus: &m.s, PrintHeaders: &m.i, Pretty: &m.p,
	}
}

func TestPrintOpts(t *testing.T) {
	cases := []struct {
		name   string
		in     meta
		method string
		want   network.PrintOpts
	}{
		{"nothing", meta{}, "GET", network.PrintOpts{}},
		{"-v shows all the metadata", meta{v: true}, "GET", network.PrintOpts{URL: true, Time: true, Status: true, Headers: true}},
		{"-u only", meta{u: true}, "GET", network.PrintOpts{URL: true}},
		{"-t only", meta{t: true}, "GET", network.PrintOpts{Time: true}},
		{"-s only", meta{s: true}, "GET", network.PrintOpts{Status: true}},
		{"-i only", meta{i: true}, "GET", network.PrintOpts{Headers: true}},
		{"-p does not turn on metadata", meta{p: true}, "GET", network.PrintOpts{Pretty: true}},
		{"-v with -p", meta{v: true, p: true}, "GET", network.PrintOpts{URL: true, Time: true, Status: true, Headers: true, Pretty: true}},
		{"POST shows nothing extra", meta{}, "POST", network.PrintOpts{}},
		{"HEAD shows status and headers", meta{}, "HEAD", network.PrintOpts{Status: true, Headers: true}},
		{"OPTIONS shows status and headers", meta{}, "OPTIONS", network.PrintOpts{Status: true, Headers: true}},
		{"HEAD still honours -u", meta{u: true}, "HEAD", network.PrintOpts{URL: true, Status: true, Headers: true}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := printOpts(c.in.flags(), c.method); got != c.want {
				t.Errorf("got %+v; want %+v", got, c.want)
			}
		})
	}
}
