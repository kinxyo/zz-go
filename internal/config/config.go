package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

type Settings struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Timeout   int    `json:"timeout"`   // seconds for a whole request; 0 means no limit
	UserAgent string `json:"userAgent"` // "" means zz/<version>, filled in by network.BuildHeaders
}

func defaults() Settings {
	return Settings{
		Host:      "127.0.0.1",
		Port:      8000,
		Timeout:   30,
		UserAgent: "",
	}
}

// Load reads the config file. If the file doesn't exist yet, it is created with the
// defaults. Keys missing from the file keep their defaults.
func Load() (Settings, error) {
	s := defaults()

	path, err := getPath()
	if err != nil {
		return s, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, write(path, s)
	}
	if err != nil {
		return s, err
	}

	if err := json.Unmarshal(data, &s); err != nil {
		return defaults(), fmt.Errorf("%s: %w", path, err)
	}

	if s.Host == "" || s.Port < 1 || s.Port > 65535 {
		return defaults(), fmt.Errorf("%s: need a host and a port between 1 and 65535", path)
	}

	if s.Timeout < 0 {
		return defaults(), fmt.Errorf("%s: timeout is a number of seconds, 0 or more (0 means no limit)", path)
	}

	return s, nil
}

// BaseURL is where paths like "/users" are sent: http://host:port.
func (s Settings) BaseURL() string {
	return "http://" + net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
}

func getPath() (string, error) {
	// ~/.config on linux, ~/Library/Application Support on mac, %AppData% on windows.
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "zz", "setting.json"), nil
}

func write(path string, s Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o644)
}
