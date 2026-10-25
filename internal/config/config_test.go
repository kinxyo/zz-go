package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolate points the config directory and the home directory at a temp dir, on every OS,
// and returns the path Load will use. Nothing touches the real config.
func isolate(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir) // linux
	t.Setenv("HOME", dir)            // macOS, and the home directory on unix
	t.Setenv("AppData", dir)         // windows
	t.Setenv("USERPROFILE", dir)     // the home directory on windows

	path, err := getPath()
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// load is Load for a request made from no directory, so no project entry is used, with the
// warnings joined into one error.
func load() (Settings, error) {
	s, _, warnings := Load("")
	return s, errors.Join(warnings...)
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

	got, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if got != defaults() {
		t.Errorf("got %+v; want the defaults %+v", got, defaults())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Load should have created %s: %v", path, err)
	}

	// what was written must load back to the same settings
	again, err := load()
	if err != nil || again != defaults() {
		t.Errorf("second Load = %+v, %v; file was:\n%s", again, err, data)
	}
}

func TestLoadReadsTheFile(t *testing.T) {
	path := isolate(t)
	writeConfig(t, path, `{"host": "example.test", "port": 3000, "timeout": 5}`)

	got, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if want := (Settings{Host: "example.test", Port: 3000, Timeout: 5}); got != want {
		t.Errorf("got %+v; want %+v", got, want)
	}
}

func TestLoadMissingKeysKeepTheirDefaults(t *testing.T) {
	path := isolate(t)

	writeConfig(t, path, `{"port": 3000}`)
	got, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if want := (Settings{Host: defaults().Host, Port: 3000, Timeout: defaults().Timeout}); got != want {
		t.Errorf("only port set: got %+v; want %+v", got, want)
	}

	writeConfig(t, path, `{"host": "example.test"}`)
	got, err = load()
	if err != nil {
		t.Fatal(err)
	}
	if want := (Settings{Host: "example.test", Port: defaults().Port, Timeout: defaults().Timeout}); got != want {
		t.Errorf("only host set: got %+v; want %+v", got, want)
	}
}

func TestLoadDoesNotOverwriteAnExistingFile(t *testing.T) {
	path := isolate(t)
	const content = `{"port": 3000}`
	writeConfig(t, path, content)

	if _, err := load(); err != nil {
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
		{"unknown key", `{"prot": 3000}`},
		{"projects isn't an object", `{"projects": []}`},
		{"a syntax error inside a project entry", `{"projects": {"/x": {"port": }}}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := isolate(t)
			writeConfig(t, path, c.content)

			got, err := load()
			if err == nil {
				t.Fatalf("want an error, got none (settings %+v)", got)
			}
			if got != defaults() {
				t.Errorf("settings = %+v; want the defaults alongside the error", got)
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

	got, err := load()
	if err != nil {
		t.Fatalf("0 means no limit and should load: %v", err)
	}
	if got.Timeout != 0 {
		t.Errorf("timeout = %d; want 0", got.Timeout)
	}
}

func TestFirstRunFileHasTheTimeout(t *testing.T) {
	path := isolate(t)

	if _, err := load(); err != nil {
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

		got, err := load()
		if err != nil {
			t.Fatal(err)
		}
		if got.UserAgent != "mine/1" {
			t.Errorf("UserAgent = %q; want mine/1", got.UserAgent)
		}
	})

	t.Run("without one in the file, it stays empty", func(t *testing.T) {
		path := isolate(t)
		writeConfig(t, path, `{"port": 3000}`)

		got, err := load()
		if err != nil {
			t.Fatal(err)
		}
		if got.UserAgent != "" { // zz/<version> is filled in when the request is built
			t.Errorf("UserAgent = %q; want empty", got.UserAgent)
		}
	})
}

func TestFirstRunFileHasAnEmptyUserAgent(t *testing.T) {
	path := isolate(t)

	if _, err := load(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"userAgent": ""`) {
		t.Errorf("new file should have an empty userAgent, not the version:\n%s", data)
	}
}

func TestFirstRunFileHasAnEmptyProjects(t *testing.T) {
	path := isolate(t)

	if _, err := load(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"projects": {}`) {
		t.Errorf("new file should show where project entries go:\n%s", data)
	}
}

// projectSetup writes a setting.json whose "projects" is projects, and makes the directory
// dir under the home directory. In projects, {home} is the home directory, written with
// forward slashes so it needs no escaping in JSON (Windows accepts them). It returns the
// directory to load from and the home directory as written in the file.
func projectSetup(t *testing.T, projects, dir string) (string, string) {
	t.Helper()

	path := isolate(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	from := filepath.Join(home, dir)
	if err := os.MkdirAll(from, 0o755); err != nil {
		t.Fatal(err)
	}

	written := filepath.ToSlash(home)
	writeConfig(t, path, `{"projects": `+strings.ReplaceAll(projects, "{home}", written)+`}`)
	return from, written
}

func TestLoadProjects(t *testing.T) {
	global := defaults()
	with := func(f func(*Settings)) Settings {
		s := defaults()
		f(&s)
		return s
	}

	cases := []struct {
		name     string
		projects string // {home} is the home directory
		dir      string // where the request is made from, under the home directory
		want     Settings
		project  string // the key that should be used; {home} as above
	}{
		{
			"an entry overrides the global settings",
			`{"{home}/api": {"host": "example.test", "port": 3000}}`, "api",
			with(func(s *Settings) { s.Host, s.Port = "example.test", 3000 }), "{home}/api",
		},
		{
			"keys an entry leaves out keep the global value",
			`{"{home}/api": {"port": 3000}}`, "api",
			with(func(s *Settings) { s.Port = 3000 }), "{home}/api",
		},
		{
			"a timeout of 0 turns the global limit off",
			`{"{home}/api": {"timeout": 0}}`, "api",
			with(func(s *Settings) { s.Timeout = 0 }), "{home}/api",
		},
		{
			"an entry covers its subdirectories",
			`{"{home}/api": {"port": 3000}}`, "api/handlers/v1",
			with(func(s *Settings) { s.Port = 3000 }), "{home}/api",
		},
		{
			"the nearest entry wins",
			`{"{home}/api": {"port": 3000}, "{home}/api/admin": {"port": 4000}}`, "api/admin/x",
			with(func(s *Settings) { s.Port = 4000 }), "{home}/api/admin",
		},
		{
			"a directory whose name only starts with the key doesn't match",
			`{"{home}/api": {"port": 3000}}`, "api-v2",
			global, "",
		},
		{
			"~/ is the home directory",
			`{"~/api": {"port": 3000}}`, "api",
			with(func(s *Settings) { s.Port = 3000 }), "~/api",
		},
		{
			"a trailing slash still matches",
			`{"{home}/api/": {"port": 3000}}`, "api",
			with(func(s *Settings) { s.Port = 3000 }), "{home}/api/",
		},
		{
			"no entry for the directory: the global settings",
			`{"{home}/web": {"port": 3000}}`, "api",
			global, "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			from, home := projectSetup(t, c.projects, c.dir)

			got, project, warnings := Load(from)
			if len(warnings) > 0 {
				t.Fatalf("warnings: %v", warnings)
			}
			if got != c.want {
				t.Errorf("settings = %+v; want %+v", got, c.want)
			}
			if want := strings.ReplaceAll(c.project, "{home}", home); project != want {
				t.Errorf("project = %q; want %q", project, want)
			}
		})
	}
}

func TestLoadProjectSymlink(t *testing.T) {
	from, home := projectSetup(t, `{"{home}/api": {"port": 3000}}`, "api")

	link := filepath.Join(filepath.Dir(from), "link")
	if err := os.Symlink(from, link); err != nil {
		t.Skip("can't make a symlink here:", err)
	}

	got, project, warnings := Load(link)
	if len(warnings) > 0 || got.Port != 3000 || project != home+"/api" {
		t.Errorf("from a symlink to the project: port %d, project %q, warnings %v", got.Port, project, warnings)
	}
}

func TestLoadProjectWarnings(t *testing.T) {
	cases := []struct {
		name     string
		projects string
		want     Settings
		project  string
		warning  string // a part of the only warning
	}{
		{
			"a relative key is ignored",
			`{"api": {"port": 3000}}`,
			defaults(), "", `projects["api"]: not an absolute path (ignored)`,
		},
		{
			"a bad value: the global settings",
			`{"{home}/api": {"port": 0}}`,
			defaults(), "", `/api"]: need a host and a port`,
		},
		{
			"an unknown key: the global settings",
			`{"{home}/api": {"prot": 3000}}`,
			defaults(), "", `unknown field "prot"`,
		},
		{
			"an entry that isn't an object: the global settings",
			`{"{home}/api": 3000}`,
			defaults(), "", "(using the global settings)",
		},
		{
			// sorted, "/..." (and "C:/..." on windows) comes before "~/...", so it is kept
			"two keys for the same directory: the first in sorted order is kept",
			`{"~/api": {"port": 3000}, "{home}/api": {"port": 4000}}`,
			Settings{Host: defaults().Host, Port: 4000, Timeout: defaults().Timeout}, "{home}/api",
			`projects["~/api"]: the same directory as projects[`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			from, home := projectSetup(t, c.projects, "api")

			got, project, warnings := Load(from)
			if len(warnings) != 1 {
				t.Fatalf("want one warning, got %v", warnings)
			}
			if !strings.Contains(warnings[0].Error(), c.warning) || !strings.Contains(warnings[0].Error(), "setting.json") {
				t.Errorf("warning %q should name the file and contain %q", warnings[0], c.warning)
			}
			if got != c.want {
				t.Errorf("settings = %+v; want %+v", got, c.want)
			}
			if want := strings.ReplaceAll(c.project, "{home}", home); project != want {
				t.Errorf("project = %q; want %q", project, want)
			}
		})
	}
}

func TestLoadProjectsOnTopOfTheFile(t *testing.T) {
	path := isolate(t)
	home, _ := os.UserHomeDir()
	writeConfig(t, path, `{"port": 3000, "timeout": 5, "projects": {"~": {"timeout": 9}}}`)

	got, _, warnings := Load(home)
	if want := (Settings{Host: defaults().Host, Port: 3000, Timeout: 9}); got != want || len(warnings) > 0 {
		t.Errorf("got %+v, %v; want %+v: the file's port, the entry's timeout", got, warnings, want)
	}
}
