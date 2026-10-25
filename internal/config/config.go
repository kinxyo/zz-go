package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Settings struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Timeout   int    `json:"timeout"`   // seconds for a whole request; 0 means no limit
	UserAgent string `json:"userAgent"` // "" means zz/<version>, filled in by network.BuildHeaders
}

// file is setting.json: the global settings at the top level, and "projects", which maps a
// directory to the settings it changes for requests made in it or below it. A project's value
// stays raw until it is used, so a key it leaves out can be told apart from a key set to zero
// ("timeout": 0 means no limit, not "keep the global timeout").
type file struct {
	Settings
	Projects map[string]json.RawMessage `json:"projects"`
}

func defaults() Settings {
	return Settings{
		Host:      "127.0.0.1",
		Port:      8000,
		Timeout:   30,
		UserAgent: "",
	}
}

// Load returns the settings for a request made from dir, and the key of the project entry
// that was used ("" for none; dir "" looks for none). The file is created with the defaults
// if it doesn't exist yet.
//
// The settings are layered: the defaults, then the file's top-level keys, then the entry in
// "projects" for dir or its nearest parent. A key that a layer leaves out keeps the value from
// the layer under it. A layer that is wrong is skipped with a warning and the layers under it
// stand: a bad file means the defaults, a bad project entry means the file's global settings.
func Load(dir string) (Settings, string, []error) {
	s := defaults()

	path, err := getPath()
	if err != nil {
		return s, "", []error{usingDefaults(err)}
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := write(path, s); err != nil {
			return s, "", []error{usingDefaults(err)}
		}
		return s, "", nil
	}
	if err != nil {
		return s, "", []error{usingDefaults(err)}
	}

	f := file{Settings: s}
	if err := decodeStrict(data, &f); err != nil {
		return s, "", []error{usingDefaults(fmt.Errorf("%s: %w", path, err))}
	}
	if err := validate(f.Settings); err != nil {
		return s, "", []error{usingDefaults(fmt.Errorf("%s: %w", path, err))}
	}
	s = f.Settings

	dirs, warnings := projectDirs(f.Projects)
	for i, w := range warnings {
		warnings[i] = fmt.Errorf("%s: %w (ignored)", path, w)
	}

	key := findProject(dirs, dir)
	if key == "" {
		return s, "", warnings
	}

	// decoded onto a copy: a value of the wrong type doesn't stop decoding, so a failed
	// entry may already have changed some fields
	next := s
	err = decodeStrict(f.Projects[key], &next)
	if err == nil {
		err = validate(next)
	}
	if err != nil {
		w := fmt.Errorf("%s: projects[%q]: %w (using the global settings)", path, key, err)
		return s, "", append(warnings, w)
	}

	return next, key, warnings
}

// BaseURL is where paths like "/users" are sent: http://host:port.
func (s Settings) BaseURL() string {
	return "http://" + net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
}

func validate(s Settings) error {
	if s.Host == "" || s.Port < 1 || s.Port > 65535 {
		return errors.New("need a host and a port between 1 and 65535")
	}

	if s.Timeout < 0 {
		return errors.New("timeout is a number of seconds, 0 or more (0 means no limit)")
	}

	return nil
}

func usingDefaults(err error) error {
	return fmt.Errorf("%w (using defaults)", err)
}

// decodeStrict is json.Unmarshal, except that a key with no field in v is an error: a typo
// like "prot" would otherwise be dropped without a word, and the request would quietly go to
// the global port.
func decodeStrict(data []byte, v any) error {
	if !json.Valid(data) {
		return json.Unmarshal(data, v) // for its message: where the syntax breaks
	}

	// valid JSON is exactly one value, so a single Decode reads all of it
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// projectDirs maps each project directory to its key as written in the file. A key may start
// with ~/ and may end with a /. It is compared as a real path (symlinks resolved), the same way
// as the directory of the request, so a project reached through a symlink still matches. A key
// that isn't an absolute path, or names a directory another key already names, is left out
// with a warning; keys are taken in sorted order so the same one is always kept.
func projectDirs(projects map[string]json.RawMessage) (map[string]string, []error) {
	keys := make([]string, 0, len(projects))
	for key := range projects {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	dirs := make(map[string]string, len(keys))
	var warnings []error

	for _, key := range keys {
		dir := expandHome(key)
		if !filepath.IsAbs(dir) {
			warnings = append(warnings, fmt.Errorf("projects[%q]: not an absolute path", key))
			continue
		}

		dir = realPath(dir)
		if kept, ok := dirs[dir]; ok {
			warnings = append(warnings, fmt.Errorf("projects[%q]: the same directory as projects[%q]", key, kept))
			continue
		}

		dirs[dir] = key
	}

	return dirs, warnings
}

// findProject returns the key for dir or its nearest parent, or "" when none has one. It walks
// up one directory at a time instead of comparing strings, so the key /x/api never matches
// /x/api-v2.
func findProject(dirs map[string]string, dir string) string {
	if dir == "" || len(dirs) == 0 {
		return ""
	}

	dir, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	dir = realPath(dir)

	for {
		if key, ok := dirs[dir]; ok {
			return key
		}

		parent := filepath.Dir(dir)
		if parent == dir { // the root, on every OS
			return ""
		}
		dir = parent
	}
}

// realPath resolves the symlinks in an absolute path. A path that doesn't exist is only
// cleaned: it can't be the directory of a request anyway.
func realPath(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return filepath.Clean(path)
}

// expandHome turns a leading ~/ into the home directory. It is the same as network's, which
// can't be used here: network imports config.
func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	return filepath.Join(home, path[1:])
}

func getPath() (string, error) {
	// ~/.config on linux, ~/Library/Application Support on mac, %AppData% on windows.
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "zz", "setting.json"), nil
}

// write creates the file with s and an empty "projects", so the file shows where project
// entries go.
func write(path string, s Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(file{Settings: s, Projects: map[string]json.RawMessage{}}, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o644)
}
