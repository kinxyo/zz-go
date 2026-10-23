package network

import "strings"

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
	// 1. zz /api/users (local, from the config)
	// 2. zz httpbin.org/json (https)
	// 3. zz -d httpbin.org/json (http)
	// A URL that already has a scheme is left alone, even with -d.

	if strings.HasPrefix(path, "/") {
		return base + path
	}

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}

	if forceHTTP {
		return "http://" + path
	}

	return "https://" + path
}
