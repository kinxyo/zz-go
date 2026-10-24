package network

import (
	"net/http"
	"strings"
	"testing"
)

func TestMatchMethod(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"get", "GET", true},
		{"g", "GET", true},
		{"GET", "GET", true},
		{"post", "POST", true},
		{"p", "POST", true},
		{"po", "POST", true},
		{"put", "PUT", true},
		{"pu", "PUT", true},
		{"patch", "PATCH", true},
		{"pa", "PATCH", true},
		{"delete", "DELETE", true},
		{"d", "DELETE", true},
		{"head", "HEAD", true},
		{"he", "HEAD", true},
		{"HEAD", "HEAD", true},
		{"options", "OPTIONS", true},
		{"opt", "OPTIONS", true},
		{"o", "OPTIONS", true},
		{"  Post ", "POST", true},
		{"trace", "", false},
		{"connect", "", false},
		{"/users", "", false},
		{"http://x.com", "", false},
		{"", "", false},
	}

	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, ok := MatchMethod(c.in)
			if got != c.want || ok != c.ok {
				t.Errorf("MatchMethod(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
			}
		})
	}
}

func TestMakeURL(t *testing.T) {
	const base = "http://127.0.0.1:8000"

	cases := []struct {
		name      string
		path      string
		forceHTTP bool
		want      string
	}{
		{"local path", "/api/users", false, "http://127.0.0.1:8000/api/users"},
		{"local path ignores -h", "/api/users", true, "http://127.0.0.1:8000/api/users"},
		{"bare host is https", "httpbin.org/json", false, "https://httpbin.org/json"},
		{"bare host with -h is http", "httpbin.org/json", true, "http://httpbin.org/json"},
		{"host and port with -h", "localhost:3000/h", true, "http://localhost:3000/h"},
		{"localhost is http", "localhost:8000/x", false, "http://localhost:8000/x"},
		{"localhost without a port", "localhost", false, "http://localhost"},
		{"localhost in capitals", "LOCALHOST:3000", false, "http://LOCALHOST:3000"},
		{"127.0.0.1 is http", "127.0.0.1:3000/x?q=1", false, "http://127.0.0.1:3000/x?q=1"},
		{"all of 127/8 is loopback", "127.0.1.1/x", false, "http://127.0.1.1/x"},
		{"ipv6 loopback with a port", "[::1]:3000/x", false, "http://[::1]:3000/x"},
		{"ipv6 loopback without a port", "[::1]/x", false, "http://[::1]/x"},
		{"a name that starts with localhost isn't local", "localhost.example.com/x", false, "https://localhost.example.com/x"},
		{"a name that starts with 127 isn't local", "127.0.0.1.nip.io/x", false, "https://127.0.0.1.nip.io/x"},
		{"a LAN address stays https", "192.168.1.5:8000/x", false, "https://192.168.1.5:8000/x"},
		{"https to localhost is kept", "https://localhost:8443/x", false, "https://localhost:8443/x"},
		{"http left alone", "http://x.com/a", false, "http://x.com/a"},
		{"http left alone with -h", "http://x.com/a", true, "http://x.com/a"},
		{"https left alone", "https://x.com/a", false, "https://x.com/a"},
		{"https left alone with -h", "https://x.com/a", true, "https://x.com/a"},
		{"query string kept", "/search?q=go&page=2", false, "http://127.0.0.1:8000/search?q=go&page=2"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Request{URL: c.path}
			r.MakeURL(base, c.forceHTTP)
			if r.URL != c.want {
				t.Errorf("MakeURL(%q, force=%v) = %q; want %q", c.path, c.forceHTTP, r.URL, c.want)
			}
		})
	}
}

func TestMakeURLUsesGivenBase(t *testing.T) {
	r := Request{URL: "/x"}
	r.MakeURL("http://[::1]:3000", false)
	if want := "http://[::1]:3000/x"; r.URL != want {
		t.Errorf("got %q; want %q", r.URL, want)
	}
}

func TestParseBody(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantBody string // what is sent
		wantType string
		wantErr  bool
	}{
		{"empty", "", "", "", false},
		{"blank", "  \n", "  \n", "", false},
		{"json object", `{"content": "hi"}`, `{"content": "hi"}`, "application/json", false},
		{"json array", `[1,2]`, `[1,2]`, "application/json", false},
		{"pretty json over several lines", "{\n  \"a\": 1\n}", "{\n  \"a\": 1\n}", "application/json", false},
		{"valid json keeps its spacing and newline", " {\"a\":1}\n", " {\"a\":1}\n", "application/json", false},
		{"unquoted key", `{content: "hi"}`, `{"content": "hi"}`, "application/json", false},
		{"unquoted key and value", `{content: hi}`, `{"content": "hi"}`, "application/json", false},
		{"ndjson", "{\"a\":1}\n{\"a\":2}\n", "{\"a\":1}\n{\"a\":2}\n", "application/x-ndjson", false},
		{"xml", "<user><name>a</name></user>", "<user><name>a</name></user>", "application/xml", false},
		{"plain text", "hello", "hello", "", false},
		{"form-like text is not detected", "x=1&y=2", "x=1&y=2", "", false},
		{"looks like json but can't be fixed", `{content: }`, `{content: }`, "", true},
		{"broken ndjson", "{\"a\":1}\n{bad", "{\"a\":1}\n{bad", "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Request{Body: c.in}
			got, err := r.ParseBody()

			if (err != nil) != c.wantErr {
				t.Fatalf("error = %v; want error: %v", err, c.wantErr)
			}
			if got != c.wantType || r.Body != c.wantBody {
				t.Errorf("got type %q body %q; want type %q body %q", got, r.Body, c.wantType, c.wantBody)
			}
		})
	}
}

func TestParseBodyErrorNamesTheMistake(t *testing.T) {
	r := Request{Body: `{content: }`}
	_, err := r.ParseBody()
	if err == nil || !strings.Contains(err.Error(), "not valid JSON") || !strings.Contains(err.Error(), "invalid character") {
		t.Errorf("error = %v; want one that says the JSON is invalid and why", err)
	}
}

func TestRelaxJSON(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{content: hi}`, `{"content": "hi"}`},
		{`{msg: hello world, n: 3, ok: true, x: null}`, `{"msg": "hello world", "n": 3, "ok": true, "x": null}`},
		{`{url: http://x.io/a?b=1, tags: [a, b, 2]}`, `{"url": "http://x.io/a?b=1", "tags": ["a", "b", 2]}`},
		{`{user: {name: alice, age: 30}}`, `{"user": {"name": "alice", "age": 30}}`},
		{`{say: he said "hi"}`, `{"say": "he said \"hi\""}`},
		{`{a: -1.5e3, b: 01}`, `{"a": -1.5e3, "b": "01"}`}, // 01 isn't a JSON number, so it stays text
		{`{1: one}`, `{"1": "one"}`}, // keys are always strings
		{`[a, b, c]`, `["a", "b", "c"]`},
		{`{a: "x, y", b: 2}`, `{"a": "x, y", "b": 2}`}, // a quoted string is copied as is
		{`{a: "esc \" quote", b: c}`, `{"a": "esc \" quote", "b": "c"}`},
		{`{id: 123}`, `{"id": 123}`},
	}

	for _, c := range cases {
		if got := relaxJSON(c.in); got != c.want {
			t.Errorf("relaxJSON(%s)\n got %s\nwant %s", c.in, got, c.want)
		}
	}
}

func TestIsNDJSON(t *testing.T) {
	cases := map[string]bool{
		"{\"a\":1}\n{\"a\":2}":     true,
		"{\"a\":1}\n\n{\"a\":2}\n": true, // blank lines are skipped
		"1\n2\n3":                  true,
		"{\"a\":1}":                false, // one line is plain JSON
		"{\n\"a\": 1\n}":           false, // pretty JSON: its lines aren't JSON alone
		"{\"a\":1}\n{bad}":         false,
		"":                         false,
	}

	for in, want := range cases {
		if got := isNDJSON(in); got != want {
			t.Errorf("isNDJSON(%q) = %v; want %v", in, got, want)
		}
	}
}

// headers runs BuildHeaders on a new Request and fails the test on an error.
func headers(t *testing.T, contentType, userAgent string, raw ...string) http.Header {
	t.Helper()

	r := Request{}
	if err := r.BuildHeaders(contentType, raw, userAgent); err != nil {
		t.Fatalf("BuildHeaders(%q, %q, %q): %v", contentType, raw, userAgent, err)
	}
	return r.Header
}

func TestBuildHeaders(t *testing.T) {
	t.Run("colons in the value survive", func(t *testing.T) {
		h := headers(t, "", "", "Authorization: Bearer a:b", "Referer: http://x.com/y")
		if got := h.Get("Authorization"); got != "Bearer a:b" {
			t.Errorf("Authorization = %q", got)
		}
		if got := h.Get("Referer"); got != "http://x.com/y" {
			t.Errorf("Referer = %q", got)
		}
	})

	t.Run("name is canonical and both sides are trimmed", func(t *testing.T) {
		h := headers(t, "", "", "x-api-key:   v  ")
		if got := h["X-Api-Key"]; len(got) != 1 || got[0] != "v" {
			t.Errorf("X-Api-Key = %q", got)
		}
	})

	t.Run("space after the colon is optional", func(t *testing.T) {
		if got := headers(t, "", "", "A:1").Get("A"); got != "1" {
			t.Errorf("A = %q", got)
		}
	})

	t.Run("empty value is allowed", func(t *testing.T) {
		if _, ok := headers(t, "", "", "X-Empty:")["X-Empty"]; !ok {
			t.Error("X-Empty is missing")
		}
	})

	t.Run("the last one wins", func(t *testing.T) {
		h := headers(t, "", "", "A: 1", "a: 2")
		if got := h["A"]; len(got) != 1 || got[0] != "2" {
			t.Errorf("A = %q; want just [2]", got)
		}
	})

	t.Run("the detected Content-Type is set", func(t *testing.T) {
		if got := headers(t, "application/json", "").Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
	})

	t.Run("no detected Content-Type sets none", func(t *testing.T) {
		if _, ok := headers(t, "", "")["Content-Type"]; ok {
			t.Error("Content-Type is set")
		}
	})

	t.Run("-H Content-Type beats the detected one", func(t *testing.T) {
		h := headers(t, "application/json", "", "content-type: text/plain")
		if got := h["Content-Type"]; len(got) != 1 || got[0] != "text/plain" {
			t.Errorf("Content-Type = %q", got)
		}
	})

	t.Run("the default User-Agent is set", func(t *testing.T) {
		if got := headers(t, "", "zz/1.2").Get("User-Agent"); got != "zz/1.2" {
			t.Errorf("User-Agent = %q", got)
		}
	})

	t.Run("-H User-Agent beats the default", func(t *testing.T) {
		h := headers(t, "", "zz/1.2", "user-agent: mine/1")
		if got := h["User-Agent"]; len(got) != 1 || got[0] != "mine/1" {
			t.Errorf("User-Agent = %q; want only mine/1", got)
		}
	})

	t.Run("an empty -H User-Agent clears it", func(t *testing.T) {
		h := headers(t, "", "zz/1.2", "User-Agent:")
		if got, ok := h["User-Agent"]; !ok || len(got) != 1 || got[0] != "" {
			t.Errorf("User-Agent = %q, present=%v; want one empty value", got, ok)
		}
	})

	t.Run("no default User-Agent sets none", func(t *testing.T) {
		if _, ok := headers(t, "", "")["User-Agent"]; ok {
			t.Error("User-Agent is set")
		}
	})

	for _, bad := range []string{"nocolon", ": value", "   : value", ""} {
		t.Run("rejects "+bad, func(t *testing.T) {
			r := Request{}
			if err := r.BuildHeaders("", []string{bad}, ""); err == nil {
				t.Errorf("want an error for %q", bad)
			}
			if r.Header != nil {
				t.Errorf("Header = %v; want it left unset on an error", r.Header)
			}
		})
	}
}
