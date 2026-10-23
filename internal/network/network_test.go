package network

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// seen is what the test server received.
type seen struct {
	method string
	header http.Header
	body   string
}

// newServer starts a server that records each request on the returned channel
// (a channel, so the test reads it without a data race) and answers with status.
func newServer(t *testing.T, status int, respBody string) (*httptest.Server, <-chan seen) {
	t.Helper()

	got := make(chan seen, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got <- seen{method: r.Method, header: r.Header.Clone(), body: string(b)}

		w.Header().Set("X-Reply", "yes")
		w.WriteHeader(status)
		io.WriteString(w, respBody)
	}))
	t.Cleanup(srv.Close)

	return srv, got
}

func TestCURLSendsRequest(t *testing.T) {
	srv, got := newServer(t, http.StatusCreated, `{"ok":true}`)

	headers := http.Header{}
	headers.Set("Authorization", "Bearer tok")
	headers.Set("Content-Type", "application/json")

	res, err := CURL("POST", srv.URL+"/items", headers, `{"a":1}`)
	if err != nil {
		t.Fatal(err)
	}

	req := <-got
	if req.method != "POST" {
		t.Errorf("server saw method %q", req.method)
	}
	if req.body != `{"a":1}` {
		t.Errorf("server saw body %q", req.body)
	}
	if req.header.Get("Authorization") != "Bearer tok" {
		t.Errorf("server saw Authorization %q", req.header.Get("Authorization"))
	}
	if req.header.Get("Content-Type") != "application/json" {
		t.Errorf("server saw Content-Type %q", req.header.Get("Content-Type"))
	}

	if res.Method != "POST" || res.URL != srv.URL+"/items" {
		t.Errorf("Result has method %q, url %q", res.Method, res.URL)
	}
	if res.StatusCode != 201 || res.Status != "201 Created" {
		t.Errorf("Result has status %d %q", res.StatusCode, res.Status)
	}
	if string(res.Body) != `{"ok":true}` {
		t.Errorf("Result has body %q", res.Body)
	}
	if res.Header.Get("X-Reply") != "yes" {
		t.Errorf("Result has no response headers: %v", res.Header)
	}
	if res.Elapsed <= 0 {
		t.Errorf("Result has Elapsed %v", res.Elapsed)
	}
}

func TestCURLWithoutHeadersOrBody(t *testing.T) {
	srv, got := newServer(t, http.StatusOK, "hi")

	res, err := CURL("GET", srv.URL, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	req := <-got
	if req.method != "GET" || req.body != "" {
		t.Errorf("server saw %q with body %q", req.method, req.body)
	}
	if ct := req.header.Get("Content-Type"); ct != "" {
		t.Errorf("a plain GET sent Content-Type %q", ct)
	}
	if string(res.Body) != "hi" {
		t.Errorf("Result has body %q", res.Body)
	}
}

func TestCURLErrorStatusIsNotAnError(t *testing.T) {
	srv, _ := newServer(t, http.StatusNotFound, "nope")

	res, err := CURL("GET", srv.URL, nil, "")
	if err != nil {
		t.Fatalf("a 404 is a response, not an error: %v", err)
	}
	if res.StatusCode != 404 || string(res.Body) != "nope" {
		t.Errorf("got %d %q", res.StatusCode, res.Body)
	}
}

func TestCURLSetsEachHeaderValue(t *testing.T) {
	srv, got := newServer(t, http.StatusOK, "")

	headers := http.Header{"Accept": {"text/plain", "application/json"}}
	if _, err := CURL("GET", srv.URL, headers, ""); err != nil {
		t.Fatal(err)
	}

	if vals := (<-got).header.Values("Accept"); len(vals) != 2 {
		t.Errorf("server saw Accept %v; want both values", vals)
	}
}

func TestCURLConnectionRefusedIsFriendly(t *testing.T) {
	srv, _ := newServer(t, http.StatusOK, "")
	url := srv.URL
	srv.Close() // nothing listens there any more

	_, err := CURL("GET", url, nil, "")
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.HasPrefix(err.Error(), "could not connect to ") {
		t.Errorf("error = %q; want the friendly message", err)
	}
}

func TestCURLBadURL(t *testing.T) {
	if _, err := CURL("GET", "http://[::1", nil, ""); err == nil {
		t.Error("want an error for a malformed URL")
	}
}

func TestCURLBadMethod(t *testing.T) {
	if _, err := CURL("BAD METHOD", "http://127.0.0.1:1/", nil, ""); err == nil {
		t.Error("want an error for an invalid method")
	}
}
