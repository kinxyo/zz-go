package network

import (
	"net/http"
	"strings"
)

// MatchMethod returns the HTTP method for m (or an alias of it), and false if m isn't one.
func MatchMethod(m string) (string, bool) {
	m = strings.ToLower(m)
	m = strings.TrimSpace(m)

	switch m {
	case "get", "g":
		return http.MethodGet, true
	case "post", "p", "po":
		return http.MethodPost, true
	case "put", "pu":
		return http.MethodPut, true
	case "patch", "pa":
		return http.MethodPatch, true
	case "delete", "d":
		return http.MethodDelete, true
	case "head", "he":
		return http.MethodHead, true
	case "options", "opt", "o":
		return http.MethodOptions, true
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
