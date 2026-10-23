package network

import "testing"

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
		{"  Post ", "POST", true},
		{"head", "", false},
		{"options", "", false},
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
		{"http left alone", "http://x.com/a", false, "http://x.com/a"},
		{"http left alone with -h", "http://x.com/a", true, "http://x.com/a"},
		{"https left alone", "https://x.com/a", false, "https://x.com/a"},
		{"https left alone with -h", "https://x.com/a", true, "https://x.com/a"},
		{"query string kept", "/search?q=go&page=2", false, "http://127.0.0.1:8000/search?q=go&page=2"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MakeURL(c.path, base, c.forceHTTP); got != c.want {
				t.Errorf("MakeURL(%q, force=%v) = %q; want %q", c.path, c.forceHTTP, got, c.want)
			}
		})
	}
}

func TestMakeURLUsesGivenBase(t *testing.T) {
	got := MakeURL("/x", "http://[::1]:3000", false)
	if want := "http://[::1]:3000/x"; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}
