package network

import (
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kinxyo/zz-go/internal/config"
	"github.com/kinxyo/zz-go/internal/terminal"
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

	res, err := (&Request{Method: "POST", URL: srv.URL + "/items", Header: headers, Body: `{"a":1}`}).CURL()
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
	if res.Proto != "HTTP/1.1" {
		t.Errorf("Result has proto %q", res.Proto)
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

	res, err := (&Request{Method: "GET", URL: srv.URL}).CURL()
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

func TestCURLEmptyUserAgentSendsNone(t *testing.T) {
	srv, got := newServer(t, http.StatusOK, "")

	headers := http.Header{"User-Agent": {""}}
	if _, err := (&Request{Method: "GET", URL: srv.URL, Header: headers}).CURL(); err != nil {
		t.Fatal(err)
	}

	if _, present := (<-got).header["User-Agent"]; present {
		t.Error("the server saw a User-Agent; an empty value should send none")
	}
}

func TestCURLErrorStatusIsNotAnError(t *testing.T) {
	srv, _ := newServer(t, http.StatusNotFound, "nope")

	res, err := (&Request{Method: "GET", URL: srv.URL}).CURL()
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
	if _, err := (&Request{Method: "GET", URL: srv.URL, Header: headers}).CURL(); err != nil {
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

	_, err := (&Request{Method: "GET", URL: url}).CURL()
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.HasPrefix(err.Error(), "could not connect to ") {
		t.Errorf("error = %q; want the friendly message", err)
	}
	if !errors.Is(err, ErrRefused) {
		t.Errorf("errors.Is(err, ErrRefused) = false for %v", err)
	}
}

func TestCURLBadURL(t *testing.T) {
	_, err := (&Request{Method: "GET", URL: "http://[::1"}).CURL()
	if err == nil {
		t.Fatal("want an error for a malformed URL")
	}
	if !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("errors.Is(err, ErrInvalidRequest) = false for %v", err)
	}
}

func TestCURLBadMethod(t *testing.T) {
	if _, err := (&Request{Method: "BAD METHOD", URL: "http://127.0.0.1:1/"}).CURL(); err == nil {
		t.Error("want an error for an invalid method")
	}
}

func TestCURLTimesOutWaitingForHeaders(t *testing.T) {

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()

	_, err := (&Request{Method: "GET", URL: srv.URL, Timeout: 50 * time.Millisecond}).CURL()
	if err == nil {
		t.Fatal("want a timeout error")
	}
	if !strings.Contains(err.Error(), "timed out after 50ms") {
		t.Errorf("error = %q; want the friendly timeout message", err)
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("errors.Is(err, ErrTimeout) = false for %v", err)
	}
}

func TestCURLTimesOutReadingTheBody(t *testing.T) {

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "start")
		w.(http.Flusher).Flush() // headers and the first bytes arrive in time
		time.Sleep(300 * time.Millisecond)
		io.WriteString(w, "end")
	}))
	defer srv.Close()

	_, err := (&Request{Method: "GET", URL: srv.URL, Timeout: 50 * time.Millisecond}).CURL()
	if err == nil {
		t.Fatal("want a timeout error")
	}
	if !strings.Contains(err.Error(), "timed out after 50ms") {
		t.Errorf("error = %q; want the friendly timeout message", err)
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("errors.Is(err, ErrTimeout) = false for %v", err)
	}
}

func TestCURLNoTimeout(t *testing.T) {
	// 0 means no limit: a reply that takes longer than a short limit still arrives
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		io.WriteString(w, "late")
	}))
	defer srv.Close()

	res, err := (&Request{Method: "GET", URL: srv.URL, Timeout: 0}).CURL()
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Body) != "late" {
		t.Errorf("body = %q", res.Body)
	}
}

func TestCURLKeepsCookiesInTheJar(t *testing.T) {
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "s-1", Path: "/"})
			return
		}
		got <- r.Header.Get("Cookie")
	}))
	t.Cleanup(srv.Close)

	jar, _ := cookiejar.New(nil)
	for _, path := range []string{"/login", "/next"} {
		if _, err := (&Request{Method: "GET", URL: srv.URL + path, Jar: jar}).CURL(); err != nil {
			t.Fatal(err)
		}
	}
	if c := <-got; c != "sid=s-1" {
		t.Errorf("Cookie = %q; want the one /login set", c)
	}
}

// prepFlags returns the flags Prepare reads: -d, -H and -m (-1 when it wasn't given).
func prepFlags(forceHTTP bool, headers []string, timeout int) *terminal.Flags {
	return &terminal.Flags{ForceHttp: &forceHTTP, RequestHeaders: &headers, Timeout: &timeout}
}

func TestPrepare(t *testing.T) {
	conf := config.Settings{UserAgent: "conf/1", Timeout: 30}

	t.Run("URL, headers in their layers, and the config's timeout", func(t *testing.T) {
		r := Request{Method: "POST", URL: "/x", Body: "{}"}
		err := r.Prepare(conf, prepFlags(false, []string{"X-Layer: cli"}, -1), Parts{
			Base:        "http://127.0.0.1:9000",
			ContentType: "application/json",
			Headers:     []string{"X-Layer: file", "X-File: 1", "User-Agent: file/1"},
		})
		if err != nil {
			t.Fatal(err)
		}

		if r.URL != "http://127.0.0.1:9000/x" || r.Timeout != 30*time.Second {
			t.Errorf("URL %q, timeout %v", r.URL, r.Timeout)
		}
		for name, want := range map[string]string{
			"X-Layer":      "cli",    // -H wins over the caller's headers
			"X-File":       "1",      // the caller's headers are sent
			"User-Agent":   "file/1", // and win over zz's defaults
			"Content-Type": "application/json",
		} {
			if got := r.Header.Get(name); got != want {
				t.Errorf("%s = %q; want %q", name, got, want)
			}
		}
	})

	t.Run("-d and -m", func(t *testing.T) {
		r := Request{Method: "GET", URL: "example.com/x"}
		if err := r.Prepare(conf, prepFlags(true, nil, 0), Parts{}); err != nil {
			t.Fatal(err)
		}
		if r.URL != "http://example.com/x" || r.Timeout != 0 || r.Header.Get("User-Agent") != "conf/1" {
			t.Errorf("URL %q, timeout %v, User-Agent %q", r.URL, r.Timeout, r.Header.Get("User-Agent"))
		}
	})

	t.Run("a bad header is an error", func(t *testing.T) {
		r := Request{Method: "GET", URL: "/x"}
		if err := r.Prepare(conf, prepFlags(false, nil, -1), Parts{Headers: []string{": x"}}); err == nil {
			t.Error("want an error")
		}
	})

	t.Run("the caller's headers aren't changed", func(t *testing.T) {
		layers := make([]string, 1, 2)
		layers[0] = "A: 1"
		r := Request{Method: "GET", URL: "/x"}
		_ = r.Prepare(conf, prepFlags(false, []string{"B: 2"}, -1), Parts{Headers: layers})
		if extra := layers[:2][1]; extra != "" {
			t.Errorf("-H was written into the caller's slice: %q", extra)
		}
	})
}
