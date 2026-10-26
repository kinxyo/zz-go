package collection

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	varPattern  = regexp.MustCompile(`\{\{(.*?)\}\}`)
	namePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

func validName(name string) bool {
	return namePattern.MatchString(name)
}

// varNames returns the names used as {{name}} in s. Something in {{ }} that isn't a valid name
// is an error: {{ token }}, with spaces, is almost always a typo, and would be sent as written.
func varNames(s string) ([]string, error) {
	var names []string
	for _, m := range varPattern.FindAllStringSubmatch(s, -1) {
		if !validName(m[1]) {
			return nil, fmt.Errorf("%q isn't a variable: a name is letters, digits, _ - and .", m[0])
		}
		names = append(names, m[1])
	}
	return names, nil
}

// scope is where the value of {{name}} comes from. The first that has it wins:
//  1. captured during this run, by login or a request above
//  2. the collection's variables
//  3. the dotenv file
//  4. the OS environment
//
// Values are JSON. The ones from the .env and the environment are always strings.
type scope struct {
	captured  map[string]json.RawMessage
	variables map[string]json.RawMessage
	dotenv    map[string]string
}

func (c *Collection) scope() *scope {
	return &scope{captured: map[string]json.RawMessage{}, variables: c.Variables, dotenv: c.env}
}

func (s *scope) lookup(name string) (json.RawMessage, bool) {
	if v, ok := s.captured[name]; ok {
		return v, true
	}
	return s.staticValue(name)
}

// static reports whether name has a value before anything is captured.
func (s *scope) static(name string) bool {
	_, ok := s.staticValue(name)
	return ok
}

func (s *scope) staticValue(name string) (json.RawMessage, bool) {
	if v, ok := s.variables[name]; ok {
		return v, true
	}
	if v, ok := s.dotenv[name]; ok {
		return encode(v), true
	}
	if v, ok := os.LookupEnv(name); ok {
		return encode(v), true
	}
	return nil, false
}

// expand fills in each {{name}} in s with its value as text. A name without a value yet stays
// as written: it is captured later in the run.
func (s *scope) expand(str string) string {
	return varPattern.ReplaceAllStringFunc(str, func(m string) string {
		v, ok := s.lookup(m[2 : len(m)-2])
		if !ok {
			return m
		}
		return text(v)
	})
}

// expandJSON fills in {{name}} in the strings of a JSON value. It works on the JSON's text, so
// key order and the way numbers are written are kept. A string that is exactly "{{name}}"
// becomes the value itself, with its type: a captured 42 is sent as 42, not "42". Anywhere
// else (in a longer string, or in a key) the value goes in as text, escaped for JSON. A name
// without a value yet stays as written.
func (s *scope) expandJSON(raw []byte) []byte {
	var out []byte

	for i := 0; i < len(raw); {
		if raw[i] != '"' {
			out = append(out, raw[i])
			i++
			continue
		}

		end := stringEnd(raw, i)
		out = append(out, s.expandString(raw[i:end], isKey(raw[end:]))...)
		i = end
	}

	return out
}

// expandString fills in one JSON string literal, quotes included.
func (s *scope) expandString(lit []byte, key bool) []byte {
	inner := string(lit[1 : len(lit)-1])

	if m := varPattern.FindStringSubmatch(inner); !key && m != nil && m[0] == inner {
		if v, ok := s.lookup(m[1]); ok {
			return v
		}
		return lit
	}

	filled := varPattern.ReplaceAllStringFunc(inner, func(m string) string {
		v, ok := s.lookup(m[2 : len(m)-2])
		if !ok {
			return m
		}
		e := encode(text(v))
		return string(e[1 : len(e)-1])
	})

	return []byte(`"` + filled + `"`)
}

// stringEnd returns the index just past the string literal that starts at raw[start].
func stringEnd(raw []byte, start int) int {
	for i := start + 1; i < len(raw); i++ {
		switch raw[i] {
		case '\\':
			i++ // the escaped character, which may be a quote
		case '"':
			return i + 1
		}
	}
	return len(raw)
}

// isKey reports whether a string literal followed by rest is an object key.
func isKey(rest []byte) bool {
	rest = bytes.TrimLeft(rest, " \t\r\n")
	return len(rest) > 0 && rest[0] == ':'
}

// text is a value as it goes into text: a URL, a header, or the inside of a string. A JSON
// string is its contents; anything else is its JSON (42, true, {"a":1}).
func text(v json.RawMessage) string {
	var s string
	if len(v) > 0 && v[0] == '"' && json.Unmarshal(v, &s) == nil {
		return s
	}
	return string(v)
}

// encode is s as a JSON string. Unlike json.Marshal, it leaves < > & as they are: the result
// is sent to an API, not put in HTML.
func encode(s string) json.RawMessage {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // a string always encodes
	return bytes.TrimSuffix(b.Bytes(), []byte("\n"))
}

// readDotenv reads a .env file.
func readDotenv(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	env, err := parseDotenv(string(data))
	if err != nil {
		return nil, fmt.Errorf("%s:%w", path, err)
	}
	return env, nil
}

// parseDotenv reads KEY=VALUE lines. Blank lines and lines starting with # are skipped, and an
// "export " in front is allowed. A value in matching quotes loses them: in double quotes \n is
// a newline, \" a quote and \\ a backslash; single quotes keep everything as written. An
// unquoted value ends at " #", a comment. The error starts with the line number and never
// shows the line, which may hold a secret.
func parseDotenv(data string) (map[string]string, error) {
	env := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(data))

	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(strings.TrimPrefix(line, "export "), "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || key == "" {
			return nil, fmt.Errorf("%d: want KEY=VALUE", n)
		}

		switch q := value; {
		case len(q) >= 2 && q[0] == '"' && q[len(q)-1] == '"':
			value = strings.NewReplacer(`\n`, "\n", `\"`, `"`, `\\`, `\`).Replace(q[1 : len(q)-1])
		case len(q) >= 2 && q[0] == '\'' && q[len(q)-1] == '\'':
			value = q[1 : len(q)-1]
		default:
			if i := strings.Index(q, " #"); i >= 0 {
				value = strings.TrimSpace(q[:i])
			}
		}

		env[key] = value
	}

	return env, sc.Err()
}
