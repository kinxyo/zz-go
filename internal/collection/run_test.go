package collection

import (
	"strings"
	"testing"
	"time"

	"github.com/kinxyo/zz-go/internal/config"
	"github.com/kinxyo/zz-go/internal/terminal"
)

// flags returns flags as the parser leaves them when none are given, changed by set.
func flags(set func(*terminal.Flags)) *terminal.Flags {
	b := func() *bool { return new(bool) }
	timeout := -1
	f := &terminal.Flags{
		Verbose: b(), PrintURL: b(), PrintTime: b(), PrintStatus: b(), PrintHeaders: b(),
		Pretty: b(), Fail: b(), ForceHttp: b(), Raw: b(), Help: b(),
		Timeout: &timeout, RequestHeaders: &[]string{},
	}
	if set != nil {
		set(f)
	}
	return f
}

func TestCheckArgs(t *testing.T) {
	if err := checkArgs([]string{"run", "api.zz.json"}, flags(nil)); err != nil {
		t.Errorf("a file and no flags: %v", err)
	}

	cases := []struct {
		args []string
		set  func(*terminal.Flags)
		want string
	}{
		{[]string{"run"}, nil, "zz run needs a collection file"},
		{[]string{"run", "a.zz.json", "b.zz.json"}, nil, "zz run takes one file; got 2"},
		{[]string{"run", "a.zz.json"}, func(f *terminal.Flags) { *f.Fail = true }, "-f doesn't apply to zz run"},
		{[]string{"run", "a.zz.json"}, func(f *terminal.Flags) { *f.Raw = true }, "-r doesn't apply to zz run"},
		{[]string{"run", "a.zz.json"}, func(f *terminal.Flags) { *f.PrintHeaders = true }, "-i doesn't apply to zz run"},
	}
	for _, c := range cases {
		err := checkArgs(c.args, flags(c.set))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("checkArgs(%q) = %v; want %q", c.args, err, c.want)
		}
	}
}

var conf = config.Settings{Host: "127.0.0.1", Port: 9000, Timeout: 30}

func TestPrepareHeaderLayers(t *testing.T) {
	c := load(t, `{
		"zz": 1,
		"headers": {"X-Layer": "collection", "X-Coll": "c", "Accept": "*/*"},
		"auth": {
			"login": {"method": "POST", "url": "/login", "headers": {"X-Layer": "login"}},
			"headers": {"X-Layer": "auth", "Authorization": "Bearer t"}
		},
		"requests": [
			{"method": "GET", "url": "/a", "headers": {"x-layer": "request"}},
			{"method": "GET", "url": "/b"},
			{"method": "GET", "url": "/c", "auth": "none"}
		]
	}`)
	sc := c.scope()

	get := func(r *Request, login bool, f *terminal.Flags) map[string]string {
		t.Helper()
		nr, err := c.prepare(r, login, sc, conf, f)
		if err != nil {
			t.Fatal(err)
		}
		h := map[string]string{}
		for name := range nr.Header {
			h[name] = nr.Header.Get(name)
		}
		return h
	}

	if h := get(c.Auth.Login, true, flags(nil)); h["X-Layer"] != "login" || h["Authorization"] != "" || h["X-Coll"] != "c" {
		t.Errorf("login gets the collection's headers and not auth's: %v", h)
	}
	if h := get(&c.Requests[0], false, flags(nil)); h["X-Layer"] != "request" || h["Authorization"] != "Bearer t" {
		t.Errorf("the request's header wins, names compared without case: %v", h)
	}
	if h := get(&c.Requests[1], false, flags(nil)); h["X-Layer"] != "auth" || h["Accept"] != "*/*" {
		t.Errorf("auth's header wins over the collection's: %v", h)
	}
	if h := get(&c.Requests[2], false, flags(nil)); h["Authorization"] != "" || h["X-Layer"] != "collection" {
		t.Errorf(`"auth": "none" sends no auth headers: %v`, h)
	}

	cli := flags(func(f *terminal.Flags) { *f.RequestHeaders = []string{"X-Layer: cli"} })
	if h := get(&c.Requests[0], false, cli); h["X-Layer"] != "cli" {
		t.Errorf("-H wins over everything: %v", h)
	}
	if h := get(c.Auth.Login, true, cli); h["X-Layer"] != "cli" {
		t.Errorf("-H reaches login too: %v", h)
	}
}

func TestPrepareURLAndBody(t *testing.T) {
	t.Setenv("ZZ_TEST_HOST", "api.example.com")

	cases := []struct {
		name, file       string
		url, body, ctype string
		set              func(*terminal.Flags)
		timeout          time.Duration
	}{
		{"/path goes to the config without a baseURL",
			`{"zz": 1, "requests": [{"method": "g", "url": "/x"}]}`,
			"http://127.0.0.1:9000/x", "", "", nil, 30 * time.Second},
		{"/path goes to baseURL when there is one",
			`{"zz": 1, "variables": {"baseURL": "http://localhost:8000/"}, "requests": [{"method": "get", "url": "/x"}]}`,
			"http://localhost:8000/x", "", "", nil, 30 * time.Second},
		{"a baseURL without a scheme follows the CLI's rules: https",
			`{"zz": 1, "variables": {"baseURL": "api.example.com"}, "requests": [{"method": "get", "url": "/x"}]}`,
			"https://api.example.com/x", "", "", nil, 30 * time.Second},
		{"a baseURL without a scheme follows the CLI's rules: localhost is http",
			`{"zz": 1, "variables": {"baseURL": "localhost:8000"}, "requests": [{"method": "get", "url": "/x"}]}`,
			"http://localhost:8000/x", "", "", nil, 30 * time.Second},
		{"variables in the URL",
			`{"zz": 1, "variables": {"id": 7}, "requests": [{"method": "GET", "url": "{{ZZ_TEST_HOST}}/p/{{id}}"}]}`,
			"https://api.example.com/p/7", "", "", nil, 30 * time.Second},
		{"-d and -m",
			`{"zz": 1, "requests": [{"method": "GET", "url": "{{ZZ_TEST_HOST}}/p"}]}`,
			"http://api.example.com/p", "", "", func(f *terminal.Flags) { *f.ForceHttp = true; *f.Timeout = 0 }, 0},
		{"a JSON body is JSON",
			`{"zz": 1, "variables": {"n": 3}, "requests": [{"method": "POST", "url": "/", "body": {"n": "{{n}}", "s": "n={{n}}"}}]}`,
			"http://127.0.0.1:9000/", `{"n":3,"s":"n=3"}`, "application/json", nil, 30 * time.Second},
		{"a string body is like a body typed on the CLI",
			`{"zz": 1, "requests": [{"method": "POST", "url": "/", "body": "{content: hi}"}]}`,
			"http://127.0.0.1:9000/", `{"content": "hi"}`, "application/json", nil, 30 * time.Second},
		{"a text body",
			`{"zz": 1, "variables": {"who": "you"}, "requests": [{"method": "POST", "url": "/", "body": "hello {{who}}"}]}`,
			"http://127.0.0.1:9000/", "hello you", "text/plain; charset=utf-8", nil, 30 * time.Second},
		{"a null body is no body",
			`{"zz": 1, "requests": [{"method": "POST", "url": "/", "body": null}]}`,
			"http://127.0.0.1:9000/", "", "", nil, 30 * time.Second},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := load(t, tc.file)
			nr, err := c.prepare(&c.Requests[0], false, c.scope(), conf, flags(tc.set))
			if err != nil {
				t.Fatal(err)
			}
			if nr.URL != tc.url || nr.Body != tc.body || nr.Header.Get("Content-Type") != tc.ctype || nr.Timeout != tc.timeout {
				t.Errorf("got url %q, body %q, Content-Type %q, timeout %v\nwant url %q, body %q, Content-Type %q, timeout %v",
					nr.URL, nr.Body, nr.Header.Get("Content-Type"), nr.Timeout, tc.url, tc.body, tc.ctype, tc.timeout)
			}
		})
	}
}
