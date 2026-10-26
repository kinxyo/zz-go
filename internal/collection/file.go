package collection

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/kinxyo/zz-go/internal/network"
)

// Collection is a .zz.json file.
type Collection struct {
	Version   int                        `json:"zz"`
	Name      string                     `json:"name"`
	Dotenv    string                     `json:"dotenv"`
	Variables map[string]json.RawMessage `json:"variables"`
	Headers   map[string]string          `json:"headers"`
	Auth      *Auth                      `json:"auth"`
	Requests  []Request                  `json:"requests"`

	path string            // the file, as it was given
	env  map[string]string // the values in the dotenv file
}

// Auth is how a collection logs in: login runs before every request, and headers go on every
// request that doesn't say "auth": "none" (never on login itself).
type Auth struct {
	Login   *Request          `json:"login"`
	Headers map[string]string `json:"headers"`
}

type Request struct {
	Name    string            `json:"name"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"` // a JSON value is sent as JSON; a string goes through ParseBody
	Auth    string            `json:"auth"` // "" or "none"
	Expect  []string          `json:"expect"`
	Capture map[string]string `json:"capture"` // variable name → subject
}

const version = 1

// Load reads a collection and checks all of it; nothing is sent. Every problem found is in the
// error, one per line, each starting with the file's name.
func Load(path string) (*Collection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	c := &Collection{path: path}
	if err := decode(data, c); err != nil {
		return nil, fmt.Errorf("%s%s", path, err)
	}

	if problems := c.validate(); len(problems) > 0 {
		errs := make([]error, len(problems))
		for i, p := range problems {
			errs[i] = fmt.Errorf("%s: %s", path, p)
		}
		return nil, errors.Join(errs...)
	}

	return c, nil
}

// decode reads the file into c, refusing keys the format doesn't have. Its error starts with
// ":line:col: " when encoding/json says where the problem is, and with ": " when it doesn't, so
// it reads right after the file name.
func decode(data []byte, c *Collection) error {
	if !json.Valid(data) {
		err := json.Unmarshal(data, c) // for its message: where the syntax breaks
		var se *json.SyntaxError
		if errors.As(err, &se) {
			line, col := position(data, se.Offset-1)
			return fmt.Errorf(":%d:%d: %s", line, col, se)
		}
		return fmt.Errorf(": %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	err := dec.Decode(c)

	var te *json.UnmarshalTypeError
	switch {
	case err == nil:
		// compacted once here, so a value put whole into a body is one line, like the rest of it
		for name, v := range c.Variables {
			c.Variables[name] = compact(v)
		}
		for i := range c.Requests {
			c.Requests[i].Body = compact(c.Requests[i].Body)
		}
		if c.Auth != nil && c.Auth.Login != nil {
			c.Auth.Login.Body = compact(c.Auth.Login.Body)
		}
		return nil

	case errors.As(err, &te):
		line, col := position(data, te.Offset-1)
		return fmt.Errorf(":%d:%d: %s: want %s, got a JSON %s", line, col, te.Field, describe(te.Type), te.Value)

	case strings.HasPrefix(err.Error(), "json: unknown field") && c.Version == 0:
		// most likely not a collection at all (package.json, a Postman export)
		return fmt.Errorf(`: no "zz" field; a collection starts with "zz": %d`, version)

	default:
		return fmt.Errorf(": %s", strings.TrimPrefix(err.Error(), "json: "))
	}
}

// position turns a byte offset into a line and a column, both from 1.
func position(data []byte, offset int64) (int, int) {
	i := int(max(0, min(offset, int64(len(data)))))
	before := data[:i]
	return bytes.Count(before, []byte("\n")) + 1, i - bytes.LastIndexByte(before, '\n')
}

// describe names a Go type the way the file's author thinks of it.
func describe(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "a string"
	case reflect.Int:
		return "a whole number"
	case reflect.Slice:
		if t.Elem().Kind() == reflect.String {
			return "an array of strings"
		}
		return "an array"
	case reflect.Map, reflect.Struct, reflect.Pointer:
		return "an object"
	}
	return t.String()
}

func compact(v json.RawMessage) json.RawMessage {
	var b bytes.Buffer
	if len(v) == 0 || json.Compact(&b, v) != nil {
		return v
	}
	return b.Bytes()
}

// validate checks everything that can be checked before sending, and returns each problem
// found. On a wrong version it stops there: the rest of the file may be another shape.
func (c *Collection) validate() []string {
	if c.Version == 0 {
		return []string{fmt.Sprintf(`no "zz" field; a collection starts with "zz": %d`, version)}
	}
	if c.Version != version {
		return []string{fmt.Sprintf(`"zz": %d isn't a version this zz reads; it reads %d`, c.Version, version)}
	}

	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	if c.Dotenv != "" {
		path := c.Dotenv
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(c.path), path)
		}

		env, err := readDotenv(path)
		if err != nil {
			add("dotenv: %s", err)
		}
		c.env = env
	}

	for _, name := range sortedKeys(c.Variables) {
		if !validName(name) {
			add("variables: %q isn't a valid name: use letters, digits, _ - and .", name)
		}
	}

	sc := c.scope()

	if v, ok := sc.lookup("baseURL"); ok {
		if s := text(v); len(v) == 0 || v[0] != '"' || s == "" || strings.HasPrefix(s, "/") {
			add(`baseURL: want a URL like "http://localhost:8000", got %s`, v)
		}
	}

	if len(c.Requests) == 0 {
		add(`no requests: "requests" is the list of requests to run`)
	}

	// Which names have a value at each point of the run. Static ones (variables, .env, the OS
	// environment) have one from the start; a captured one from the request that captures it on.
	captured := map[string]bool{}
	notYet := func(name string) string {
		return fmt.Sprintf("{{%s}} isn't defined: it's not in variables, the .env or the environment", name)
	}

	// collection headers go on login too, before anything is captured
	for _, p := range checkHeaders(c.Headers) {
		add("headers: %s", p)
	}
	for _, name := range usedIn(c.Headers) {
		if !sc.static(name) {
			add("headers: %s (headers are sent on login too, before anything is captured)", notYet(name))
		}
	}

	if c.Auth != nil && c.Auth.Login != nil {
		login := c.Auth.Login
		for _, p := range c.checkRequest(login, sc) {
			add("auth.login: %s", p)
		}
		if login.Auth != "" {
			add(`auth.login: "auth" doesn't apply to login`)
		}
		for _, name := range login.used() {
			if !sc.static(name) {
				add("auth.login: %s", notYet(name))
			}
		}
		for name := range login.Capture {
			captured[name] = true
		}
	}

	if c.Auth != nil {
		for _, p := range checkHeaders(c.Auth.Headers) {
			add("auth.headers: %s", p)
		}
		for _, name := range usedIn(c.Auth.Headers) {
			if !sc.static(name) && !captured[name] {
				add("auth.headers: %s, and login doesn't capture it", notYet(name))
			}
		}
	}

	for i := range c.Requests {
		r := &c.Requests[i]
		label := c.label(i)

		for _, p := range c.checkRequest(r, sc) {
			add("%s: %s", label, p)
		}
		if r.Auth != "" && r.Auth != "none" {
			add(`%s: "auth": %q: the only value is "none" (no auth headers on this request)`, label, r.Auth)
		}

		for _, name := range r.used() {
			if sc.static(name) || captured[name] {
				continue
			}
			if j := c.capturedFrom(name, i); j == i {
				add("%s: {{%s}} is used in the request that captures it", label, name)
			} else if j > i {
				add("%s: {{%s}} is used before %s captures it", label, name, c.label(j))
			} else {
				add("%s: %s, and nothing above captures it", label, notYet(name))
			}
		}

		for name := range r.Capture {
			captured[name] = true
		}
	}

	return problems
}

// checkRequest checks the parts of a request that stand on their own: method, url, headers,
// body, expect and capture. Variables are checked by validate, which knows the order.
func (c *Collection) checkRequest(r *Request, sc *scope) []string {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	if strings.TrimSpace(r.Method) == "" {
		add(`"method" is missing`)
	} else if _, ok := network.MatchMethod(r.Method); !ok {
		add("method %q isn't one zz knows", r.Method)
	}

	if strings.TrimSpace(r.URL) == "" {
		add(`"url" is missing`)
	}

	for _, p := range checkHeaders(r.Headers) {
		add("headers: %s", p)
	}

	for _, s := range []string{r.URL, string(r.Body)} {
		if _, err := varNames(s); err != nil {
			add("%s", err)
		}
	}

	// A string body is checked like a body typed on the CLI, when its value is known now. One
	// that uses a captured value is checked when it's sent.
	if body, ok := r.textBody(); ok {
		body = sc.expand(body)
		if names, _ := varNames(body); len(names) == 0 {
			nr := network.Request{Body: body}
			if _, err := nr.ParseBody(); err != nil {
				add("%s", err) // "body is not valid JSON: …"
			}
		}
	}

	for _, e := range r.Expect {
		if _, err := parseExpect(e); err != nil {
			add("expect %q: %s", e, err)
		}
	}

	for _, name := range sortedKeys(r.Capture) {
		if !validName(name) {
			add("capture %q: not a valid variable name: use letters, digits, _ - and .", name)
		}
		if _, err := parseCapture(r.Capture[name]); err != nil {
			add("capture %q: %s", name, err)
		}
	}

	return problems
}

// capturedFrom returns the first request from i on that captures name, or -1 if none does.
func (c *Collection) capturedFrom(name string, i int) int {
	for j := i; j < len(c.Requests); j++ {
		if _, ok := c.Requests[j].Capture[name]; ok {
			return j
		}
	}
	return -1
}

// checkHeaders checks header names and values. A name is compared without case, so "Accept"
// and "accept" in the same object would race for the same header.
func checkHeaders(h map[string]string) []string {
	var problems []string
	seen := map[string]string{}

	for _, name := range sortedKeys(h) {
		if !validHeaderName(name) {
			problems = append(problems, fmt.Sprintf("%q isn't a valid header name", name))
			continue
		}
		if first, ok := seen[strings.ToLower(name)]; ok {
			problems = append(problems, fmt.Sprintf("%q and %q are the same header", first, name))
		}
		seen[strings.ToLower(name)] = name

		if strings.ContainsAny(h[name], "\r\n") {
			problems = append(problems, fmt.Sprintf("the value of %q has a line break", name))
		}
		if _, err := varNames(h[name]); err != nil {
			problems = append(problems, err.Error())
		}
	}

	return problems
}

// validHeaderName is the token rule of RFC 9110: no spaces, colons or other separators.
func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r <= ' ' || r >= 0x7f || strings.ContainsRune(`"(),/:;<=>?@[\]{}`, r) {
			return false
		}
	}
	return true
}

// textBody returns the body when it's a JSON string, decoded.
func (r *Request) textBody() (string, bool) {
	var s string
	if len(r.Body) == 0 || r.Body[0] != '"' || json.Unmarshal(r.Body, &s) != nil {
		return "", false
	}
	return s, true
}

// used returns the variable names a request uses in its url, headers and body, each once.
func (r *Request) used() []string {
	parts := []string{r.URL, string(r.Body)}
	for _, name := range sortedKeys(r.Headers) {
		parts = append(parts, r.Headers[name])
	}
	return namesIn(parts...)
}

func usedIn(h map[string]string) []string {
	var values []string
	for _, name := range sortedKeys(h) {
		values = append(values, h[name])
	}
	return namesIn(values...)
}

// namesIn returns the valid variable names used in parts, in order, each once. Malformed ones
// are reported where the parts are checked.
func namesIn(parts ...string) []string {
	var names []string
	seen := map[string]bool{}

	for _, p := range parts {
		for _, m := range varPattern.FindAllStringSubmatch(p, -1) {
			if name := m[1]; validName(name) && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	return names
}

// label names request i in messages: by position from 1, and by name when it has one.
func (c *Collection) label(i int) string {
	if name := c.Requests[i].Name; name != "" {
		return fmt.Sprintf("request %d (%s)", i+1, name)
	}
	return fmt.Sprintf("request %d", i+1)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
