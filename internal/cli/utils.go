package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// TESTONLY: only the tests call this until @file is wired back in.
// expandHome turns a leading ~/ into the home directory. The shell does that for a word that
// starts with ~, but not for the part after an @.
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

// TESTONLY: only the tests call this until @file is wired back in.
// describeBody names where a body came from, for error messages.
func describeBody(arg string) string {
	switch {
	case arg == "-":
		return "stdin"
	case strings.HasPrefix(arg, "@"):
		return fmt.Sprintf("file %q", arg[1:])
	default:
		return fmt.Sprintf("payload %q", arg)
	}
}

// TESTONLY: only the tests call this until @file is wired back in.
// resolveBody returns the body for an argument that is inline, "@path" (a file) or "-" (stdin).
func resolveBody(arg string, stdin io.Reader) (string, error) {
	body := arg

	switch {
	case arg == "-":
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		body = string(b)

	case strings.HasPrefix(arg, "@"):
		b, err := os.ReadFile(expandHome(arg[1:]))
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("reading the body: %w (to send text that starts with @, pipe it in with -)", err)
		}
		if err != nil {
			return "", fmt.Errorf("reading the body: %w", err)
		}
		body = string(b)
	}

	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("%s is empty", describeBody(arg))
	}

	return body, nil
}
