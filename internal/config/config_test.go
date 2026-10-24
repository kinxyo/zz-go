package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolate points the config directory at a temp dir, on every OS, and returns
// the path Load will use. Nothing touches the real config.
func isolate(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir) // linux
	t.Setenv("HOME", dir)            // macOS
	t.Setenv("AppData", dir)         // windows

	path, err := getPath()
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// testUA is the User-Agent the tests pass to Load, the way main passes zz/<version>.
const testUA = "zz/test"

// want is the defaults as Load returns them: with the User-Agent it was given.
func want() Settings {
	s := defaults()
	s.UserAgent = testUA
	return s
}

func writeConfig(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFirstRunCreatesTheFile(t *testing.T) {
	path := isolate(t)

	got, err := Load(testUA)
	if err != nil {
		t.Fatal(err)
	}
	if got != want() {
		t.Errorf("got %+v; want the defaults %+v", got, want())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Load should have created %s: %v", path, err)
	}

	// what was written must load back to the same settings
	again, err := Load(testUA)
	if err != nil || again != want() {
		t.Errorf("second Load = %+v, %v; file was:\n%s", again, err, data)
	}
}

func TestLoadReadsTheFile(t *testing.T) {
	path := isolate(t)
	writeConfig(t, path, `{"host": "example.test", "port": 3000, "timeout": 5}`)

	got, err := Load(testUA)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Settings{Host: "example.test", Port: 3000, Timeout: 5, UserAgent: testUA}); got != want {
		t.Errorf("got %+v; want %+v", got, want)
	}
}

func TestLoadMissingKeysKeepTheirDefaults(t *testing.T) {
	path := isolate(t)

	writeConfig(t, path, `{"port": 3000}`)
	got, err := Load(testUA)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Settings{Host: defaults().Host, Port: 3000, Timeout: defaults().Timeout, UserAgent: testUA}); got != want {
		t.Errorf("only port set: got %+v; want %+v", got, want)
	}

	writeConfig(t, path, `{"host": "example.test"}`)
	got, err = Load(testUA)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Settings{Host: "example.test", Port: defaults().Port, Timeout: defaults().Timeout, UserAgent: testUA}); got != want {
		t.Errorf("only host set: got %+v; want %+v", got, want)
	}
}

func TestLoadDoesNotOverwriteAnExistingFile(t *testing.T) {
	path := isolate(t)
	const content = `{"port": 3000}`
	writeConfig(t, path, content)

	if _, err := Load(testUA); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != content {
		t.Errorf("file changed to:\n%s", data)
	}
}

func TestLoadRejectsBadFiles(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"not json", `{bad`},
		{"port as a string", `{"port": "8000"}`},
		{"port zero", `{"port": 0}`},
		{"port negative", `{"port": -1}`},
		{"port too big", `{"port": 99999}`},
		{"empty host", `{"host": ""}`},
		{"empty file", ``},
		{"negative timeout", `{"timeout": -1}`},
		{"timeout as a duration string", `{"timeout": "30s"}`},
		{"fractional timeout", `{"timeout": 1.5}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := isolate(t)
			writeConfig(t, path, c.content)

			got, err := Load(testUA)
			if err == nil {
				t.Fatalf("want an error, got none (settings %+v)", got)
			}
			if got != want() {
				t.Errorf("settings = %+v; want the defaults, with the User-Agent, alongside the error", got)
			}
			if !strings.Contains(err.Error(), "setting.json") {
				t.Errorf("error %q should name the file", err)
			}
		})
	}
}

func TestBaseURL(t *testing.T) {
	cases := []struct {
		settings Settings
		want     string
	}{
		{Settings{Host: "127.0.0.1", Port: 8000}, "http://127.0.0.1:8000"},
		{Settings{Host: "localhost", Port: 80}, "http://localhost:80"},
		{Settings{Host: "::1", Port: 3000}, "http://[::1]:3000"},
		{defaults(), "http://127.0.0.1:8000"},
	}

	for _, c := range cases {
		if got := c.settings.BaseURL(); got != c.want {
			t.Errorf("%+v.BaseURL() = %q; want %q", c.settings, got, c.want)
		}
	}
}

func TestTimeoutDefaultAndZero(t *testing.T) {
	if defaults().Timeout != 30 {
		t.Errorf("default timeout = %d; want 30", defaults().Timeout)
	}

	path := isolate(t)
	writeConfig(t, path, `{"timeout": 0}`)

	got, err := Load(testUA)
	if err != nil {
		t.Fatalf("0 means no limit and should load: %v", err)
	}
	if got.Timeout != 0 {
		t.Errorf("timeout = %d; want 0", got.Timeout)
	}
}

func TestFirstRunFileHasTheTimeout(t *testing.T) {
	path := isolate(t)

	if _, err := Load(testUA); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"timeout": 30`) {
		t.Errorf("new file should show the timeout:\n%s", data)
	}
}

func TestLoadUserAgent(t *testing.T) {
	t.Run("the file's userAgent wins", func(t *testing.T) {
		path := isolate(t)
		writeConfig(t, path, `{"userAgent": "mine/1"}`)

		got, err := Load(testUA)
		if err != nil {
			t.Fatal(err)
		}
		if got.UserAgent != "mine/1" {
			t.Errorf("UserAgent = %q; want mine/1", got.UserAgent)
		}
	})

	t.Run("without one in the file, the given one is used", func(t *testing.T) {
		path := isolate(t)
		writeConfig(t, path, `{"port": 3000}`)

		got, err := Load(testUA)
		if err != nil {
			t.Fatal(err)
		}
		if got.UserAgent != testUA {
			t.Errorf("UserAgent = %q; want %q", got.UserAgent, testUA)
		}
	})
}
