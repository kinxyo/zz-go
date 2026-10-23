package network

import (
	"fmt"
	"strings"
)

// MatchMethod returns the HTTP method for m (or an alias of it), and false if m isn't one.
func MatchMethod(m string) (string, bool) {
	m = strings.ToLower(m)
	m = strings.TrimSpace(m)

	switch m {
	case "get", "g":
		return "GET", true
	case "post", "p", "po":
		return "POST", true
	case "put", "pu":
		return "PUT", true
	case "patch", "pa":
		return "PATCH", true
	case "delete", "d":
		return "DELETE", true
	default:
		return "", false
	}
}

// MakeURL turns what the user typed into a URL. base is where paths like "/users" go.
func MakeURL(path string, base string, forceHTTP bool) string {
	// Example:
	// 1. zap /api/users (local)
	// 2. zap httpbin.org/json (global)
	// 3. zap -d httpbin.org/json (global without https)

	if strings.HasPrefix(path, "/") {
		return base + path
	}

	if forceHTTP {
		return fmt.Sprintf("http://%s", path)
	}

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return fmt.Sprintf("%s", path)
	}

	return fmt.Sprintf("https://%s", path)
}
