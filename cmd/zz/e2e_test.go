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
	"strings"
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
	Auth        string  `json:"auth"`
	UserAgent   *string `json:"user_agent"` // nil when no User-Agent was sent
	Body        string  `json:"body"`
}

const jsonBody = `{"ok":true,"n":[1,2]}`

func newTestServer(t *testing.T) *httptest.Server {
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
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		e := echoed{
			Method:      r.Method,
			ContentType: r.Header.Get("Content-Type"),
			Auth:        r.Header.Get("Authorization"),
			Body:        string(body),
		}
		if ua, ok := r.Header["User-Agent"]; ok {
			s := strings.Join(ua, ",")
			e.UserAgent = &s
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(e)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// ---- running zz -----------------------------------------------------------------------------

// env is one isolated setup: a test server, and a directory used as HOME and as the config dir.
type env struct {
	dir string
	srv *httptest.Server
}

func newEnv(t *testing.T) *env {
	t.Helper()
	if zzBin == "" {
		t.Skip("end-to-end tests are skipped with -short")
	}
	t.Parallel()

	return &env{dir: t.TempDir(), srv: newTestServer(t)}
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

// run runs zz with the given stdin (none when empty) and arguments.
func (e *env) run(t *testing.T, stdin string, args ...string) result {
	t.Helper()

	cmd := exec.Command(zzBin, args...)
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

	t.Run("-v shows the url, time, status and headers", func(t *testing.T) {
		r := e.run(t, "", "-v", "/json")
		expect(t, r, 0, jsonBody)
		if !strings.HasPrefix(r.stderr, "GET "+e.srv.URL+"/json\nTime: ") {
			t.Errorf("stderr should start with the url and time:\n%s", r.stderr)
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

	t.Run("plain text is sent as is, with no Content-Type", func(t *testing.T) {
		got := e.echo(t, "", "post", "/echo", "hello")
		if got.ContentType != "" || got.Body != "hello" {
			t.Errorf("server saw %+v", got)
		}
	})

	t.Run("-r sends the body as typed, with no Content-Type", func(t *testing.T) {
		got := e.echo(t, "", "-r", "post", "/echo", `{name: Alice}`)
		if got.ContentType != "" || got.Body != `{name: Alice}` {
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
