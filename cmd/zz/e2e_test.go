package main

// End-to-end tests. They build the real zz binary once and run it as a separate process
// against a local test server, checking what a user sees: stdout, stderr and the exit code,
// and what the server received. Each test gets its own server and its own config directory.
//
// `go test -short` skips them. What they can't cover is anything that needs a real terminal
// (colors, the newline added on a terminal, refusing binary output); those are covered by
// unit tests of the helpers that decide them.

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

const e2eVersion = "9.9.9"

var zzBin string // the built binary; empty when the end-to-end tests are skipped

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}

	dir, err := os.MkdirTemp("", "zz-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	zzBin = filepath.Join(dir, "zz")
	if runtime.GOOS == "windows" {
		zzBin += ".exe"
	}

	build := exec.Command("go", "build", "-ldflags", "-X github.com/kinxyo/zz-go/internal/config.Version="+e2eVersion, "-o", zzBin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building zz: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// ---- the test server ------------------------------------------------------------------------

// echoed is what /echo answers with: what the server received.
type echoed struct {
	Method      string  `json:"method"`
	ContentType string  `json:"content_type"`
	HasCT       bool    `json:"has_content_type"` // false when no Content-Type was sent, not even an empty one
	Auth        string  `json:"auth"`
	UserAgent   *string `json:"user_agent"` // nil when no User-Agent was sent
	Body        string  `json:"body"`
}

const jsonBody = `{"ok":true,"n":[1,2]}`

// received is one request the test server got.
type received struct {
	Method, Path string
	Header       http.Header
	Body         string
}

// serverLog is every request the test server got, in order.
type serverLog struct {
	mu   sync.Mutex
	reqs []received
}

func (l *serverLog) all() []received {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.reqs)
}

func newTestServer(t *testing.T) (*httptest.Server, *serverLog) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, jsonBody)
	})
	mux.HandleFunc("/text", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, "hello") // no newline: the bytes must arrive as they are
	})
	mux.HandleFunc("/404", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, "nope")
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
			io.WriteString(w, "late")
		case <-r.Context().Done():
		}
	})
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Password string }
		json.NewDecoder(r.Body).Decode(&in)
		if in.Password != "hunter2" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "s-1", Path: "/"})
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"token": "tok-1"}`)
	})
	mux.HandleFunc("POST /items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/items/42")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id": 42}`)
	})
	mux.HandleFunc("/items/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id": 42}`)
	})
	mux.HandleFunc("/echo/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, r.URL.Path)
	})
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		e := echoed{
			Method:      r.Method,
			ContentType: r.Header.Get("Content-Type"),
			Auth:        r.Header.Get("Authorization"),
			Body:        string(body),
		}
		_, e.HasCT = r.Header["Content-Type"]
		if ua, ok := r.Header["User-Agent"]; ok {
			s := strings.Join(ua, ",")
			e.UserAgent = &s
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(e)
	})

	log := &serverLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(strings.NewReader(string(body)))

		log.mu.Lock()
		log.reqs = append(log.reqs, received{r.Method, r.URL.Path, r.Header.Clone(), string(body)})
		log.mu.Unlock()

		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, log
}

// ---- running zz -----------------------------------------------------------------------------

// env is one isolated setup: a test server, and a directory used as HOME and as the config dir.
type env struct {
	dir string
	srv *httptest.Server
	log *serverLog // what srv received
}

func newEnv(t *testing.T) *env {
	t.Helper()
	if zzBin == "" {
		t.Skip("end-to-end tests are skipped with -short")
	}
	t.Parallel()

	srv, log := newTestServer(t)
	return &env{dir: t.TempDir(), srv: srv, log: log}
}

// configPath is where zz looks for setting.json, given the variables run sets.
func (e *env) configPath() string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(e.dir, "Library", "Application Support", "zz", "setting.json")
	}
	return filepath.Join(e.dir, "zz", "setting.json") // XDG_CONFIG_HOME on linux, AppData on windows
}

// config writes a setting.json pointing at the test server, so /path URLs reach it.
// extra is added inside the object, e.g. `, "timeout": 1`.
func (e *env) config(t *testing.T, extra string) {
	t.Helper()

	u, _ := url.Parse(e.srv.URL)
	e.writeConfig(t, fmt.Sprintf(`{"host": %q, "port": %s%s}`, u.Hostname(), u.Port(), extra))
}

func (e *env) writeConfig(t *testing.T, content string) {
	t.Helper()

	path := e.configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

type result struct {
	stdout, stderr string
	code           int
	took           time.Duration
}

// run runs zz in e.dir with the given stdin (none when empty) and arguments.
func (e *env) run(t *testing.T, stdin string, args ...string) result {
	t.Helper()

	cmd := exec.Command(zzBin, args...)
	cmd.Dir = e.dir // never the repo, so nothing around it can be taken for a project
	cmd.Env = []string{
		"HOME=" + e.dir, "USERPROFILE=" + e.dir, // home directory
		"XDG_CONFIG_HOME=" + e.dir, "AppData=" + e.dir, // config directory
		"NO_COLOR=1",
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	start := time.Now()
	err := cmd.Run()
	r := result{stdout: stdout.String(), stderr: stderr.String(), took: time.Since(start)}

	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		r.code = exitErr.ExitCode()
	case err != nil:
		t.Fatalf("running zz %q: %v", args, err)
	}
	return r
}

// echo runs zz against /echo and returns what the server received.
func (e *env) echo(t *testing.T, stdin string, args ...string) echoed {
	t.Helper()

	r := e.run(t, stdin, args...)
	if r.code != 0 {
		t.Fatalf("zz %q exited %d\nstderr: %s", args, r.code, r.stderr)
	}

	var got echoed
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("the reply isn't the server's JSON: %v\nstdout: %q\nstderr: %q", err, r.stdout, r.stderr)
	}
	return got
}

func expect(t *testing.T, r result, code int, stdout string) {
	t.Helper()

	if r.code != code {
		t.Errorf("exit %d; want %d\nstderr: %s", r.code, code, r.stderr)
	}
	if r.stdout != stdout {
		t.Errorf("stdout = %q; want %q", r.stdout, stdout)
	}
}

func contains(t *testing.T, what, got string, parts ...string) {
	t.Helper()

	for _, p := range parts {
		if !strings.Contains(got, p) {
			t.Errorf("%s should contain %q; got:\n%s", what, p, got)
		}
	}
}

// ---- tests ----------------------------------------------------------------------------------

func TestE2EOutput(t *testing.T) {
	e := newEnv(t)
	e.config(t, "")

	t.Run("the body is written byte for byte, with nothing on stderr", func(t *testing.T) {
		r := e.run(t, "", "/json")
		expect(t, r, 0, jsonBody)
		if r.stderr != "" {
			t.Errorf("stderr = %q; want nothing", r.stderr)
		}
	})

	t.Run("no newline is added when stdout isn't a terminal", func(t *testing.T) {
		expect(t, e.run(t, "", "/text"), 0, "hello")
	})

	t.Run("-p pretty-prints JSON", func(t *testing.T) {
		expect(t, e.run(t, "", "-p", "/json"), 0, "{\n  \"ok\": true,\n  \"n\": [\n    1,\n    2\n  ]\n}\n")
	})

	t.Run("-p leaves a non-JSON body alone", func(t *testing.T) {
		expect(t, e.run(t, "", "-p", "/text"), 0, "hello")
	})

	t.Run("-s puts the status on stderr, then a blank line", func(t *testing.T) {
		r := e.run(t, "", "-s", "/json")
		expect(t, r, 0, jsonBody)
		if r.stderr != "HTTP/1.1 200 OK\n\n" {
			t.Errorf("stderr = %q", r.stderr)
		}
	})

	t.Run("-i lists the response headers, sorted", func(t *testing.T) {
		r := e.run(t, "", "-i", "/json")
		expect(t, r, 0, jsonBody)
		contains(t, "stderr", r.stderr, "Content-Length: 21\n", "Content-Type: application/json\n", "Date: ")
		if !(strings.Index(r.stderr, "Content-Length") < strings.Index(r.stderr, "Content-Type") &&
			strings.Index(r.stderr, "Content-Type") < strings.Index(r.stderr, "Date")) {
			t.Errorf("headers aren't sorted:\n%s", r.stderr)
		}
	})

	t.Run("-v shows the config used, the url, time, status and headers", func(t *testing.T) {
		r := e.run(t, "", "-v", "/json")
		expect(t, r, 0, jsonBody)
		if !strings.HasPrefix(r.stderr, "config: no project entry\nGET "+e.srv.URL+"/json\nTime: ") {
			t.Errorf("stderr should start with the config, the url and time:\n%s", r.stderr)
		}
		contains(t, "stderr", r.stderr, "HTTP/1.1 200 OK\n", "Content-Type: application/json\n")
	})

	t.Run("head shows the status and headers without being asked, and no body", func(t *testing.T) {
		r := e.run(t, "", "head", "/json")
		expect(t, r, 0, "")
		contains(t, "stderr", r.stderr, "HTTP/1.1 200 OK\n", "Content-Type: application/json\n")
	})

	t.Run("a 404 is a response: body printed, exit 0", func(t *testing.T) {
		expect(t, e.run(t, "", "/404"), 0, "nope")
	})
}

func TestE2ERequest(t *testing.T) {
	e := newEnv(t)
	e.config(t, "")

	t.Run("inline JSON is posted with a JSON Content-Type", func(t *testing.T) {
		got := e.echo(t, "", "post", "/echo", `{"a":1}`)
		if got.Method != "POST" || got.ContentType != "application/json" || got.Body != `{"a":1}` {
			t.Errorf("server saw %+v", got)
		}
	})

	t.Run("a body without a method stays a GET", func(t *testing.T) {
		if got := e.echo(t, "", "/echo", `{"a":1}`); got.Method != "GET" {
			t.Errorf("method = %q", got.Method)
		}
	})

	t.Run("an explicit method wins", func(t *testing.T) {
		if got := e.echo(t, "", "get", "/echo", `{"a":1}`); got.Method != "GET" {
			t.Errorf("method = %q", got.Method)
		}
	})

	t.Run("-H reaches the server", func(t *testing.T) {
		if got := e.echo(t, "", "-H", "Authorization: Bearer tok", "/echo"); got.Auth != "Bearer tok" {
			t.Errorf("Authorization = %q", got.Auth)
		}
	})

	t.Run("the default User-Agent carries the version", func(t *testing.T) {
		got := e.echo(t, "", "/echo")
		if got.UserAgent == nil || *got.UserAgent != "zz/"+e2eVersion {
			t.Errorf("User-Agent = %s; want zz/%s", show(got.UserAgent), e2eVersion)
		}
	})

	t.Run("-H replaces the User-Agent, and an empty one sends none", func(t *testing.T) {
		if got := e.echo(t, "", "-H", "User-Agent: mine/1", "/echo"); got.UserAgent == nil || *got.UserAgent != "mine/1" {
			t.Errorf("User-Agent = %s; want mine/1", show(got.UserAgent))
		}
		if got := e.echo(t, "", "-H", "User-Agent:", "/echo"); got.UserAgent != nil {
			t.Errorf("User-Agent = %q; want none", *got.UserAgent)
		}
	})

	t.Run("a non-JSON body goes through with your own Content-Type", func(t *testing.T) {
		got := e.echo(t, "", "-H", "Content-Type: application/x-www-form-urlencoded", "post", "/echo", "user=a&pass=b")
		if got.ContentType != "application/x-www-form-urlencoded" || got.Body != "user=a&pass=b" {
			t.Errorf("server saw %+v", got)
		}
	})

	t.Run("loose JSON is fixed up and sent as JSON", func(t *testing.T) {
		got := e.echo(t, "", "post", "/echo", `{name: Alice, age: 30}`)
		if got.ContentType != "application/json" || got.Body != `{"name": "Alice", "age": 30}` {
			t.Errorf("server saw %+v", got)
		}
	})

	t.Run("valid JSON is sent exactly as typed", func(t *testing.T) {
		if got := e.echo(t, "", "post", "/echo", ` {"a" : 1} `); got.Body != ` {"a" : 1} ` {
			t.Errorf("body = %q", got.Body)
		}
	})

	t.Run("@file sends the file exactly, final newline included", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "body.json")
		os.WriteFile(path, []byte("{\"name\":\"Alice\"}\n"), 0o644)

		got := e.echo(t, "", "post", "/echo", "@"+path)
		if got.Body != "{\"name\":\"Alice\"}\n" || got.ContentType != "application/json" {
			t.Errorf("server saw %+v", got)
		}
	})

	t.Run("@file with loose JSON is fixed up like a typed body", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "loose.json")
		os.WriteFile(path, []byte("{name: Alice}\n"), 0o644)

		if got := e.echo(t, "", "post", "/echo", "@"+path); got.Body != `{"name": "Alice"}` {
			t.Errorf("body = %q", got.Body)
		}
	})

	t.Run("@~/ is the home directory", func(t *testing.T) {
		os.WriteFile(filepath.Join(e.dir, "home-body.json"), []byte(`{"from":"home"}`), 0o644)

		if got := e.echo(t, "", "post", "/echo", "@~/home-body.json"); got.Body != `{"from":"home"}` {
			t.Errorf("body = %q", got.Body)
		}
	})

	t.Run("-r still reads @file, and sends it untouched", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "loose.json")
		os.WriteFile(path, []byte("{name: Alice}\n"), 0o644)

		got := e.echo(t, "", "-r", "post", "/echo", "@"+path)
		if got.Body != "{name: Alice}\n" || got.HasCT {
			t.Errorf("server saw %+v", got)
		}
	})

	t.Run("plain text is sent as is, as text/plain", func(t *testing.T) {
		got := e.echo(t, "", "post", "/echo", "hello")
		if got.ContentType != "text/plain; charset=utf-8" || got.Body != "hello" {
			t.Errorf("server saw %+v", got)
		}
	})

	t.Run("a request without a body sends no Content-Type", func(t *testing.T) {
		if got := e.echo(t, "", "/echo"); got.HasCT {
			t.Errorf("Content-Type = %q; want none", got.ContentType)
		}
	})

	t.Run("-r sends the body as typed, with no Content-Type", func(t *testing.T) {
		got := e.echo(t, "", "-r", "post", "/echo", `{name: Alice}`)
		if got.HasCT || got.Body != `{name: Alice}` {
			t.Errorf("server saw %+v", got)
		}
	})

	t.Run("flags work after the arguments", func(t *testing.T) {
		got := e.echo(t, "", "post", "/echo", `{"a":1}`, "-H", "Authorization: Bearer tok")
		if got.Auth != "Bearer tok" || got.Body != `{"a":1}` {
			t.Errorf("server saw %+v", got)
		}
	})

	t.Run("a local host typed without a scheme is reached over http", func(t *testing.T) {
		u, _ := url.Parse(e.srv.URL) // 127.0.0.1:<port>
		if got := e.echo(t, "", "p", u.Host+"/echo", `{content: hi}`); got.Method != "POST" || got.Body != `{"content": "hi"}` {
			t.Errorf("server saw %+v", got)
		}
	})
}

func TestE2EExitCodes(t *testing.T) {
	e := newEnv(t)
	e.config(t, "")

	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL
	closed.Close() // nothing listens there any more

	tlsSrv := httptest.NewTLSServer(http.NotFoundHandler())
	defer tlsSrv.Close()

	plainAsHTTPS := "https://" + strings.TrimPrefix(e.srv.URL, "http://") + "/json"

	cases := []struct {
		name   string
		args   []string
		code   int
		stderr string // a part of the message
	}{
		{"success", []string{"/json"}, 0, ""},
		{"missing URL", []string{"get", ""}, 2, "missing URL"},
		{"bad body", []string{"post", "/echo", "{content: }"}, 2, "not valid JSON"},
		{"missing @file", []string{"post", "/echo", "@nope.json"}, 2, "reading the body"},
		{"bad -H", []string{"-H", "nocolon", "/json"}, 2, `invalid value "nocolon"`},
		{"unknown flag", []string{"-z", "/json"}, 2, "flag provided but not defined: -z"},
		{"bad -m", []string{"-m", "1.5", "/json"}, 2, "whole number of seconds"},
		{"malformed URL", []string{"http://[::1"}, 2, "missing ']'"},
		{"connection refused", []string{closedURL}, 7, "connection refused"},
		{"-f on a 404", []string{"-f", "/404"}, 22, ""},
		{"https to a plain HTTP server", []string{plainAsHTTPS}, 35, "plain HTTP"},
		{"untrusted certificate", []string{tlsSrv.URL}, 60, "TLS certificate"},
		{"no such host", []string{"-m", "10", "http://no-such-host.invalid/"}, 6, "no such host"},
		// 23 (binary refused on a terminal) needs a real terminal; see the note at the top.
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := e.run(t, "", c.args...)
			if r.code != c.code {
				t.Errorf("zz %q exited %d; want %d\nstderr: %s", c.args, r.code, c.code, r.stderr)
			}
			contains(t, "stderr", r.stderr, c.stderr)
		})
	}
}

func TestE2ETimeout(t *testing.T) {
	e := newEnv(t)

	t.Run("the config's timeout applies", func(t *testing.T) {
		e.config(t, `, "timeout": 1`)
		r := e.run(t, "", "/slow")
		if r.code != 28 || !strings.Contains(r.stderr, "timed out after 1s") {
			t.Errorf("exit %d, stderr %q", r.code, r.stderr)
		}
		if r.took > 1900*time.Millisecond {
			t.Errorf("took %v; the limit was 1s", r.took)
		}
	})

	t.Run("-m overrides the config", func(t *testing.T) {
		e.config(t, `, "timeout": 0`)
		r := e.run(t, "", "-m", "1", "/slow")
		if r.code != 28 || !strings.Contains(r.stderr, "timed out after 1s") {
			t.Errorf("exit %d, stderr %q", r.code, r.stderr)
		}
	})

	t.Run("-m 0 means no limit", func(t *testing.T) {
		e.config(t, `, "timeout": 1`)
		expect(t, e.run(t, "", "-m", "0", "/slow"), 0, "late")
	})
}

func TestE2EConfig(t *testing.T) {
	e := newEnv(t)

	t.Run("version doesn't create the config", func(t *testing.T) {
		expect(t, e.run(t, "", "version"), 0, "zz -- v"+e2eVersion+"\n")
		if _, err := os.Stat(e.configPath()); err == nil {
			t.Error("zz version created the config file")
		}
	})

	t.Run("the first request creates it with the defaults", func(t *testing.T) {
		expect(t, e.run(t, "", e.srv.URL+"/json"), 0, jsonBody)

		data, err := os.ReadFile(e.configPath())
		if err != nil {
			t.Fatalf("no config file after a request: %v", err)
		}
		contains(t, "setting.json", string(data), `"host": "127.0.0.1"`, `"port": 8000`, `"timeout": 30`)
	})

	t.Run("a bad config warns and uses the defaults", func(t *testing.T) {
		e.writeConfig(t, `{"timeout": -1}`)
		r := e.run(t, "", e.srv.URL+"/json")
		expect(t, r, 0, jsonBody)
		contains(t, "stderr", r.stderr, "config:", "(using defaults)")
	})

	t.Run("host and port decide where /path goes", func(t *testing.T) {
		e.config(t, "")
		expect(t, e.run(t, "", "/json"), 0, jsonBody)
	})

	t.Run("a project entry for the directory wins over the global host and port", func(t *testing.T) {
		closed := httptest.NewServer(http.NotFoundHandler())
		closedPort := strings.TrimPrefix(closed.URL, "http://127.0.0.1:")
		closed.Close() // the global settings point where nothing listens

		u, _ := url.Parse(e.srv.URL)
		e.writeConfig(t, fmt.Sprintf(`{"port": %s, "projects": {%q: {"port": %s}}}`, closedPort, e.dir, u.Port()))

		r := e.run(t, "", "-v", "/json")
		expect(t, r, 0, jsonBody)
		contains(t, "stderr", r.stderr, "config: project "+e.dir+"\n")
	})
}

// file writes a file in e.dir.
func (e *env) file(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(e.dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// squash turns each run of spaces into one, for output whose column widths vary.
func squash(s string) string {
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

// paths lists the method and path of each request the server got.
func paths(reqs []received) []string {
	var out []string
	for _, r := range reqs {
		out = append(out, r.Method+" "+r.Path)
	}
	return out
}

// loginCollection logs in with the password in .env, captures from the response, and has a
// request that can't connect in the middle.
const loginCollection = `{
	"zz": 1,
	"name": "e2e api",
	"dotenv": ".env",
	"auth": {
		"login": {"method": "POST", "url": "/login", "body": {"password": "{{PASSWORD}}"}, "capture": {"token": "json.token"}},
		"headers": {"Authorization": "Bearer {{token}}"}
	},
	"requests": [
		{"name": "create", "method": "POST", "url": "/items", "body": {"z": 1, "a": 2}, "capture": {"id": "json.id", "loc": "header Location"}},
		{"name": "read", "method": "GET", "url": "{{loc}}"},
		{"method": "GET", "url": "/echo/{{id}}", "auth": "none"},
		{"name": "refused", "method": "GET", "url": "%s/x"},
		{"name": "after", "method": "DELETE", "url": "/echo"}
	]
}`

func TestE2ERun(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL
	closed.Close() // nothing listens there any more

	t.Run("login, captures, the cookie jar and -H, and a refused request doesn't stop the run", func(t *testing.T) {
		e := newEnv(t)
		e.config(t, "")
		e.file(t, ".env", "PASSWORD=hunter2\n")
		e.file(t, "api.zz.json", fmt.Sprintf(loginCollection, closedURL))

		r := e.run(t, "", "run", "api.zz.json", "-H", "X-Trace: 1")
		if r.code != 1 {
			t.Errorf("exit %d; want 1 (one request failed)\nstderr: %s", r.code, r.stderr)
		}
		// the refused URL's port sets the width of the url column; the skip test checks the columns
		contains(t, "stdout", squash(r.stdout),
			"e2e api\n\n",
			"✓ login POST /login 200 ",
			"✓ create POST /items 201 ",
			"✓ read GET {{loc}} 200 ",
			"✓ GET /echo/{{id}} 200 ",
			"✗ refused GET "+closedURL+"/x could not connect to ",
			"✓ after DELETE /echo 200 ",
			"\n5 passed, 1 failed, 0 skipped\n",
		)

		got := e.log.all()
		want := []string{"POST /login", "POST /items", "GET /items/42", "GET /echo/42", "DELETE /echo"}
		if !slices.Equal(paths(got), want) {
			t.Fatalf("the server got %q; want %q", paths(got), want)
		}

		login, create, noAuth, after := got[0], got[1], got[3], got[4]
		if login.Body != `{"password":"hunter2"}` || login.Header.Get("Authorization") != "" {
			t.Errorf("login: body %q, Authorization %q; want the .env password and no auth header", login.Body, login.Header.Get("Authorization"))
		}
		if create.Header.Get("Authorization") != "Bearer tok-1" || create.Body != `{"z":1,"a":2}` || create.Header.Get("Content-Type") != "application/json" {
			t.Errorf("create: Authorization %q, body %q, Content-Type %q", create.Header.Get("Authorization"), create.Body, create.Header.Get("Content-Type"))
		}
		if noAuth.Header.Get("Authorization") != "" {
			t.Errorf(`"auth": "none" sent Authorization %q`, noAuth.Header.Get("Authorization"))
		}
		if after.Header.Get("Authorization") != "Bearer tok-1" {
			t.Errorf("the request after the refused one: Authorization %q", after.Header.Get("Authorization"))
		}
		for _, req := range got {
			if req.Header.Get("X-Trace") != "1" {
				t.Errorf("%s %s: no -H X-Trace", req.Method, req.Path)
			}
			if c := req.Header.Get("Cookie"); req.Path != "/login" && c != "sid=s-1" {
				t.Errorf("%s %s: Cookie %q; want login's sid=s-1", req.Method, req.Path, c)
			}
		}
	})

	t.Run("a failed login stops the run", func(t *testing.T) {
		e := newEnv(t)
		e.config(t, "")
		e.file(t, ".env", "PASSWORD=wrong\n")
		e.file(t, "api.zz.json", fmt.Sprintf(loginCollection, closedURL))

		r := e.run(t, "", "run", "api.zz.json")
		if r.code != 1 {
			t.Errorf("exit %d; want 1", r.code)
		}
		contains(t, "stdout", squash(r.stdout),
			"✗ login POST /login 401 ",
			"\n login needs a 2xx status; got 401 Unauthorized\n",
			"\n capture token: response isn't JSON\n",
			"login failed, so the 5 requests after it weren't sent\n",
		)
		if got := paths(e.log.all()); !slices.Equal(got, []string{"POST /login"}) {
			t.Errorf("the server got %q; want only the login", got)
		}
	})

	t.Run("a capture that finds nothing fails its request and skips the ones that need it", func(t *testing.T) {
		e := newEnv(t)
		e.config(t, "")
		e.file(t, "skip.zz.json", `{"zz": 1, "requests": [
			{"name": "make", "method": "POST", "url": "/items", "capture": {"id": "json.nope"}},
			{"name": "use", "method": "GET", "url": "/echo/{{id}}"},
			{"name": "other", "method": "GET", "url": "/json"}
		]}`)

		r := e.run(t, "", "run", "skip.zz.json")
		if r.code != 1 {
			t.Errorf("exit %d; want 1", r.code)
		}
		contains(t, "stdout", r.stdout,
			"✗ make   POST /items        201 ",
			"    capture id: json.nope not found\n",
			"- use    GET  /echo/{{id}}  skipped: needs {{id}} from make\n",
			"✓ other  GET  /json         200 ",
			"1 passed, 1 failed, 1 skipped\n",
		)
		if got := paths(e.log.all()); !slices.Equal(got, []string{"POST /items", "GET /json"}) {
			t.Errorf("the server got %q; the skipped request was sent", got)
		}
	})

	t.Run("every request passing exits 0; -v shows the config, the URL and the response, -p pretty", func(t *testing.T) {
		e := newEnv(t)
		e.config(t, "")
		e.file(t, "ok.zz.json", `{"zz": 1, "requests": [{"method": "GET", "url": "/json"}]}`)

		r := e.run(t, "", "run", "ok.zz.json")
		expect(t, r, 0, r.stdout)
		contains(t, "stdout", r.stdout, "ok.zz.json\n\n✓ GET /json  200 ", "1 passed, 0 failed, 0 skipped\n")
		if strings.Contains(r.stdout, "HTTP/1.1") {
			t.Errorf("without -v there are no response details:\n%s", r.stdout)
		}

		r = e.run(t, "", "run", "ok.zz.json", "-v", "-p")
		expect(t, r, 0, r.stdout)
		contains(t, "stdout", r.stdout,
			"config: no project entry\n",
			"    GET "+e.srv.URL+"/json\n    HTTP/1.1 200 OK\n",
			"    Content-Type: application/json\n",
			"\n    {\n      \"ok\": true,\n      \"n\": [\n",
		)
	})

	t.Run("a problem with the file or the arguments exits 2, and nothing is sent", func(t *testing.T) {
		e := newEnv(t)
		e.config(t, "")
		e.file(t, "api.zz.json", `{"zz": 1, "requests": [{"method": "GET", "url": "/json"}]}`)
		e.file(t, "broken.zz.json", `{"zz": 1,`)
		e.file(t, "invalid.zz.json", `{"zz": 1, "requests": [{"method": "GRAB", "url": "/"}, {"method": "GET", "url": "/json"}]}`)

		for _, c := range []struct {
			name   string
			args   []string
			stderr string
		}{
			{"no file", []string{"run"}, "zz run needs a collection file"},
			{"a file that isn't there", []string{"run", "nope.zz.json"}, "open nope.zz.json: no such file or directory"},
			{"a flag that doesn't apply", []string{"run", "api.zz.json", "-i"}, "-i doesn't apply to zz run"},
			{"-p without -v", []string{"run", "api.zz.json", "-p"}, "-p pretty-prints the bodies -v shows; add -v"},
			{"a bad -H", []string{"run", "api.zz.json", "-H", ": x"}, `header ": x": want "Key: Value"`},
			{"a broken file", []string{"run", "broken.zz.json"}, "broken.zz.json:1:"},
			{"a file that doesn't validate", []string{"run", "invalid.zz.json"}, "invalid.zz.json: request 1: method \"GRAB\" isn't one zz knows\n"},
		} {
			t.Run(c.name, func(t *testing.T) {
				r := e.run(t, "", c.args...)
				expect(t, r, 2, "")
				contains(t, "stderr", r.stderr, c.stderr)
			})
		}

		if got := e.log.all(); len(got) > 0 {
			t.Errorf("the server got %q; want nothing", paths(got))
		}
	})
}

func TestE2EHelpAndVersion(t *testing.T) {
	e := newEnv(t)

	help := e.run(t, "", "-h")
	expect(t, help, 0, help.stdout)
	contains(t, "help", help.stdout, "zigzag (zz) "+e2eVersion)

	for _, args := range [][]string{{"-help"}, {"--help"}, {"help"}, {"-h", "/json"}} {
		if r := e.run(t, "", args...); r.code != 0 || r.stdout != help.stdout {
			t.Errorf("zz %q: exit %d, and the output differs from zz -h", args, r.code)
		}
	}

	for _, args := range [][]string{{"version"}, {"v"}} {
		expect(t, e.run(t, "", args...), 0, "zz -- v"+e2eVersion+"\n")
	}
}

// show prints an optional header value: (none) when it wasn't sent.
func show(s *string) string {
	if s == nil {
		return "(none)"
	}
	return fmt.Sprintf("%q", *s)
}
