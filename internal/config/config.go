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
	Host string `json:"host"`
	Port int    `json:"port"`
}

func defaults() Settings {
	return Settings{
		Host: "127.0.0.1",
		Port: 8000,
	}
}

// Load returns the settings from the config file. If the file doesn't exist yet,
// it is created with the defaults. Keys missing from the file keep their defaults.
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
