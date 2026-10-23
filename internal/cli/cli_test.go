package cli

import (
	"net/http"
	"testing"

	"github.com/kinxyo/zz-go/internal/config"
	"github.com/kinxyo/zz-go/internal/network"
	"github.com/kinxyo/zz-go/internal/terminal"
)

var local = config.Settings{Host: "127.0.0.1", Port: 8000}

// flagsFor builds only the flags extract reads.
func flagsFor(forceHTTP bool, headers ...string) *terminal.Flags {
	return &terminal.Flags{ForceHttp: &forceHTTP, Headers: &headers}
}

func TestExtract(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		flags    *terminal.Flags
		settings config.Settings
		method   string
		url      string
		payload  string
	}{
		{"bare path", []string{"/x"}, flagsFor(false), local, "GET", "http://127.0.0.1:8000/x", ""},
		{"bare host is https", []string{"httpbin.org/json"}, flagsFor(false), local, "GET", "https://httpbin.org/json", ""},
		{"method and url", []string{"get", "/x"}, flagsFor(false), local, "GET", "http://127.0.0.1:8000/x", ""},
		{"method is case insensitive", []string{"POST", "/x"}, flagsFor(false), local, "POST", "http://127.0.0.1:8000/x", ""},
		{"alias with body", []string{"p", "/x", `{"a":1}`}, flagsFor(false), local, "POST", "http://127.0.0.1:8000/x", `{"a":1}`},
		{"body without a method", []string{"/x", `{"a":1}`}, flagsFor(false), local, "GET", "http://127.0.0.1:8000/x", `{"a":1}`},
		{"array body", []string{"post", "/x", `[1,2]`}, flagsFor(false), local, "POST", "http://127.0.0.1:8000/x", `[1,2]`},
		{"string body", []string{"post", "/x", `"hi"`}, flagsFor(false), local, "POST", "http://127.0.0.1:8000/x", `"hi"`},
		{"-h on a bare host", []string{"x.com/a"}, flagsFor(true), local, "GET", "http://x.com/a", ""},
		{"-h leaves a full https url", []string{"https://x.com/a"}, flagsFor(true), local, "GET", "https://x.com/a", ""},
		{"settings decide the local base", []string{"/x"}, flagsFor(false), config.Settings{Host: "localhost", Port: 3000}, "GET", "http://localhost:3000/x", ""},
		{"ipv6 host in settings", []string{"/x"}, flagsFor(false), config.Settings{Host: "::1", Port: 3000}, "GET", "http://[::1]:3000/x", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := extract(c.args, c.flags, c.settings, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.method != c.method || got.url != c.url || got.payload != c.payload {
				t.Errorf("got method=%q url=%q payload=%q\nwant method=%q url=%q payload=%q",
					got.method, got.url, got.payload, c.method, c.url, c.payload)
			}
		})
	}
}

func TestExtractErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"method only", []string{"get"}, "missing URL"},
		{"empty body", []string{"post", "/x", ""}, `payload "" is not valid JSON`},
		{"text body", []string{"post", "/x", "hello"}, `payload "hello" is not valid JSON`},
		{"truncated json", []string{"post", "/x", `{"a":`}, `payload "{\"a\":" is not valid JSON`},
		{"flag after the url", []string{"/x", "-v"}, `"-v" looks like a flag; flags go before the arguments`},
		{"flag after the method", []string{"get", "-v", "/x"}, `"-v" looks like a flag; flags go before the arguments`},
		{"flag after a short method", []string{"he", "-u", "/x"}, `"-u" looks like a flag; flags go before the arguments`},
		{"unknown method then url", []string{"trace", "http://x.com/y"}, `unknown method "trace" (use get, post, put, patch, delete, head or options)`},
		{"unknown method then path", []string{"purge", "/y"}, `unknown method "purge" (use get, post, put, patch, delete, head or options)`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := extract(c.args, flagsFor(false), local, "")
			if err == nil {
				t.Fatalf("want error %q, got none", c.want)
			}
			if err.Error() != c.want {
				t.Errorf("error = %q; want %q", err, c.want)
			}
		})
	}
}

func TestExtractHeaders(t *testing.T) {
	got, err := extract([]string{"post", "/x", `{"a":1}`}, flagsFor(false, "Authorization: Bearer tok"), local, "")
	if err != nil {
		t.Fatal(err)
	}

	if got.headers.Get("Authorization") != "Bearer tok" {
		t.Errorf("Authorization = %q", got.headers.Get("Authorization"))
	}
	if got.headers.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q; want the default for a body", got.headers.Get("Content-Type"))
	}
}

func TestExtractBadHeader(t *testing.T) {
	if _, err := extract([]string{"/x"}, flagsFor(false, "nocolon"), local, ""); err == nil {
		t.Error("want an error for a header without a colon")
	}
}

func TestBuildHeaders(t *testing.T) {
	t.Run("colons in the value survive", func(t *testing.T) {
		h := mustBuild(t, "", "Authorization: Bearer a:b", "Referer: http://x.com/y")
		if got := h.Get("Authorization"); got != "Bearer a:b" {
			t.Errorf("Authorization = %q", got)
		}
		if got := h.Get("Referer"); got != "http://x.com/y" {
			t.Errorf("Referer = %q", got)
		}
	})

	t.Run("name is canonical and both sides are trimmed", func(t *testing.T) {
		h := mustBuild(t, "", "x-api-key:   v  ")
		if got := h["X-Api-Key"]; len(got) != 1 || got[0] != "v" {
			t.Errorf("X-Api-Key = %q", got)
		}
	})

	t.Run("space after the colon is optional", func(t *testing.T) {
		if got := mustBuild(t, "", "A:1").Get("A"); got != "1" {
			t.Errorf("A = %q", got)
		}
	})

	t.Run("empty value is allowed", func(t *testing.T) {
		h := mustBuild(t, "", "X-Empty:")
		if _, ok := h["X-Empty"]; !ok {
			t.Errorf("X-Empty is missing from %v", h)
		}
	})

	t.Run("the last one wins", func(t *testing.T) {
		h := mustBuild(t, "", "A: 1", "a: 2")
		if got := h["A"]; len(got) != 1 || got[0] != "2" {
			t.Errorf("A = %q; want just [2]", got)
		}
	})

	t.Run("no body means no default Content-Type", func(t *testing.T) {
		if got := mustBuild(t, "").Get("Content-Type"); got != "" {
			t.Errorf("Content-Type = %q", got)
		}
	})

	t.Run("a body adds Content-Type json", func(t *testing.T) {
		if got := mustBuild(t, `{"a":1}`).Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
	})

	t.Run("the user's Content-Type beats the default", func(t *testing.T) {
		h := mustBuild(t, `{"a":1}`, "content-type: text/plain")
		if got := h["Content-Type"]; len(got) != 1 || got[0] != "text/plain" {
			t.Errorf("Content-Type = %q", got)
		}
	})

	for _, bad := range []string{"nocolon", ": value", "   : value", ""} {
		t.Run("rejects "+bad, func(t *testing.T) {
			if _, err := buildHeaders("", []string{bad}, ""); err == nil {
				t.Errorf("want an error for %q", bad)
			}
		})
	}
}

func mustBuild(t *testing.T, payload string, raw ...string) http.Header {
	t.Helper()

	h, err := buildHeaders(payload, raw, "")
	if err != nil {
		t.Fatalf("buildHeaders(%q, %q): %v", payload, raw, err)
	}
	return h
}

func TestLooksLikeURL(t *testing.T) {
	cases := map[string]bool{
		"/users":            true,
		"http://x.com":      true,
		"https://x.com/a?b": true,
		`{"a":1}`:           false,
		"hello":             false,
		"-v":                false,
		"x.com/a":           false,
		"":                  false,
	}

	for in, want := range cases {
		if got := looksLikeURL(in); got != want {
			t.Errorf("looksLikeURL(%q) = %v; want %v", in, got, want)
		}
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

func TestFailed(t *testing.T) {
	cases := []struct {
		status int
		fail   bool
		want   bool
	}{
		{200, true, false},
		{204, true, false},
		{301, true, false},
		{399, true, false},
		{400, true, true},
		{404, true, true},
		{500, true, true},
		{404, false, false},
		{500, false, false},
	}

	for _, c := range cases {
		if got := failed(c.status, c.fail); got != c.want {
			t.Errorf("failed(%d, %v) = %v; want %v", c.status, c.fail, got, c.want)
		}
	}
}

func TestExtractEmptyURL(t *testing.T) {
	for _, args := range [][]string{{""}, {"  "}, {"get", ""}, {"post", "   "}} {
		_, err := extract(args, flagsFor(false), local, "")
		if err == nil || err.Error() != "missing URL" {
			t.Errorf("extract(%q) error = %v; want \"missing URL\"", args, err)
		}
	}
}

func TestExtractNewMethods(t *testing.T) {
	cases := map[string]string{"head": "HEAD", "he": "HEAD", "options": "OPTIONS", "opt": "OPTIONS", "o": "OPTIONS"}

	for word, want := range cases {
		got, err := extract([]string{word, "/x"}, flagsFor(false), local, "")
		if err != nil {
			t.Errorf("%s: %v", word, err)
			continue
		}
		if got.method != want || got.url != "http://127.0.0.1:8000/x" {
			t.Errorf("%s: got method=%q url=%q; want %q", word, got.method, got.url, want)
		}
	}
}

func TestUserAgent(t *testing.T) {
	t.Run("default is set", func(t *testing.T) {
		h, err := buildHeaders("", nil, "zz/1.2")
		if err != nil {
			t.Fatal(err)
		}
		if got := h.Get("User-Agent"); got != "zz/1.2" {
			t.Errorf("User-Agent = %q", got)
		}
	})

	t.Run("-H overrides it", func(t *testing.T) {
		h, err := buildHeaders("", []string{"user-agent: mine/1"}, "zz/1.2")
		if err != nil {
			t.Fatal(err)
		}
		if got := h["User-Agent"]; len(got) != 1 || got[0] != "mine/1" {
			t.Errorf("User-Agent = %q; want only mine/1", got)
		}
	})

	t.Run("empty default sets nothing", func(t *testing.T) {
		h, _ := buildHeaders("", nil, "")
		if _, ok := h["User-Agent"]; ok {
			t.Errorf("User-Agent present: %v", h)
		}
	})

	t.Run("reaches the request through extract", func(t *testing.T) {
		r, err := extract([]string{"/x"}, flagsFor(false), local, "zz/1.2")
		if err != nil {
			t.Fatal(err)
		}
		if got := r.headers.Get("User-Agent"); got != "zz/1.2" {
			t.Errorf("User-Agent = %q", got)
		}
	})
}
