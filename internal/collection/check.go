package collection

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kinxyo/zz-go/internal/network"
)

// subject is what an expectation or a capture reads from a response.
type subject struct {
	kind   string   // "status", "text", "header" or "json"
	header string   // for "header": its name
	path   []string // for "json": the keys and indexes after "json.", none for the whole body
}

// expectation is one entry of "expect", parsed. "status 201" and "status 2xx" have the op
// "==" and the value 201 or "2xx".
type expectation struct {
	subject subject
	op      string
	value   json.RawMessage // nil for "exists"
}

var ops = map[string]bool{"==": true, "!=": true, ">": true, "<": true, ">=": true, "<=": true, "contains": true, "exists": true}

const opList = "==, !=, >, <, >=, <=, contains or exists"

// parseExpect reads `<subject> <op> [value]`, or `status <code or class>`. The value is the
// rest of the string: a JSON value if it parses as one (12, true, null, "quoted"), text if not.
func parseExpect(s string) (expectation, error) {
	subj, rest, err := parseSubject(strings.TrimSpace(s))
	if err != nil {
		return expectation{}, err
	}

	if subj.kind == "status" {
		v, err := parseStatus(rest)
		return expectation{subject: subj, op: "==", value: v}, err
	}

	op, value, _ := strings.Cut(rest, " ")
	value = strings.TrimSpace(value)

	switch {
	case op == "":
		return expectation{}, errors.New("missing an operator: " + opList)
	case !ops[op]:
		return expectation{}, fmt.Errorf("%q isn't an operator: use %s", op, opList)
	case op == "exists" && value != "":
		return expectation{}, errors.New(`"exists" takes no value`)
	case op == "exists":
		return expectation{subject: subj, op: op}, nil
	case value == "":
		return expectation{}, fmt.Errorf("%q needs a value to compare with", op)
	}

	v := json.RawMessage(value)
	if !json.Valid(v) {
		v = encode(value)
	}

	if strings.ContainsAny(op, "<>") && !isNumber(v) {
		return expectation{}, fmt.Errorf("%s compares numbers, and %s isn't one", op, value)
	}

	return expectation{subject: subj, op: op, value: v}, nil
}

// parseStatus reads a code (201) or a class (2xx).
func parseStatus(s string) (json.RawMessage, error) {
	s = strings.TrimSpace(s)

	if n, err := strconv.Atoi(s); err == nil && n >= 100 && n <= 599 {
		return json.RawMessage(s), nil
	}
	if len(s) == 3 && s[0] >= '1' && s[0] <= '5' && strings.ToLower(s[1:]) == "xx" {
		return encode(strings.ToLower(s)), nil
	}

	return nil, errors.New("want a code like 201 or a class like 2xx")
}

// parseCapture reads a capture's subject: json.<path> (the value keeps its JSON type) or
// header <Name> (a string).
func parseCapture(s string) (subject, error) {
	subj, rest, err := parseSubject(strings.TrimSpace(s))
	if err != nil {
		return subject{}, err
	}

	if (subj.kind != "json" && subj.kind != "header") || rest != "" {
		return subject{}, errors.New("a capture is json.<path> or header <Name>")
	}

	return subj, nil
}

// parseSubject reads the subject at the start of s and returns what follows it.
func parseSubject(s string) (subject, string, error) {
	word, rest, _ := strings.Cut(s, " ")
	rest = strings.TrimSpace(rest)

	switch {
	case word == "status" || word == "text":
		return subject{kind: word}, rest, nil

	case word == "header":
		name, rest, _ := strings.Cut(rest, " ")
		if name == "" {
			return subject{}, "", errors.New(`"header" needs a name: header Content-Type …`)
		}
		if !validHeaderName(name) {
			return subject{}, "", fmt.Errorf("%q isn't a valid header name", name)
		}
		return subject{kind: "header", header: name}, strings.TrimSpace(rest), nil

	case word == "json":
		return subject{kind: "json"}, rest, nil

	case strings.HasPrefix(word, "json."):
		path := strings.Split(word[len("json."):], ".")
		for _, key := range path {
			if key == "" {
				return subject{}, "", fmt.Errorf("%q: a path is keys and indexes joined by dots, like json.items.0.id", word)
			}
		}
		return subject{kind: "json", path: path}, rest, nil
	}

	return subject{}, "", fmt.Errorf("%q isn't something zz can check: use status, text, header <Name> or json.<path>", word)
}

// String is the subject as written in the file: json.items.0.id, header Location.
func (s subject) String() string {
	switch s.kind {
	case "header":
		return "header " + s.header
	case "json":
		return strings.Join(append([]string{"json"}, s.path...), ".")
	}
	return s.kind
}

// read returns the subject's value in a response, as JSON: the status is a number, the text and
// a header are strings, and a json path is the value found there, with its type.
func (s subject) read(res *network.Result) (json.RawMessage, error) {
	switch s.kind {
	case "status":
		return json.RawMessage(strconv.Itoa(res.StatusCode)), nil

	case "text":
		return encode(string(res.Body)), nil

	case "header":
		values := res.Header.Values(s.header)
		if len(values) == 0 {
			return nil, fmt.Errorf("%s not found", s)
		}
		return encode(strings.Join(values, ", ")), nil
	}

	if !json.Valid(res.Body) {
		return nil, errors.New("response isn't JSON")
	}

	v := json.RawMessage(res.Body)
	for i, key := range s.path {
		next, ok := step(v, key, i == len(s.path)-1)
		if !ok {
			return nil, fmt.Errorf("%s not found", s)
		}
		v = next
	}

	return compact(v), nil
}

// step returns what key names in v: an object's key, an array's index, or, as the last key,
// "length": the number of elements of an array, keys of an object, or characters of a string. An
// object's own "length" key wins over its length.
func step(v json.RawMessage, key string, last bool) (json.RawMessage, bool) {
	length := func(n int) (json.RawMessage, bool) {
		if !last || key != "length" {
			return nil, false
		}
		return json.RawMessage(strconv.Itoa(n)), true
	}

	switch v = trimSpace(v); v[0] {
	case '{':
		var m map[string]json.RawMessage
		_ = json.Unmarshal(v, &m) // v is valid JSON
		if x, ok := m[key]; ok {
			return x, true
		}
		return length(len(m))

	case '[':
		var a []json.RawMessage
		_ = json.Unmarshal(v, &a)
		if n, ok := index(key); ok && n < len(a) {
			return a[n], true
		}
		return length(len(a))

	case '"':
		return length(utf8.RuneCountInString(text(v)))
	}

	return nil, false
}

// index reads an array index: digits only, so "+1" and "-0" are keys.
func index(key string) (int, bool) {
	for _, c := range key {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(key)
	return n, err == nil
}

func trimSpace(v json.RawMessage) json.RawMessage {
	return json.RawMessage(strings.TrimSpace(string(v)))
}

// capture takes each of caps from res into sc, and returns a problem for each one that found
// nothing. The ones that did are kept either way.
func (sc *scope) capture(caps map[string]string, res *network.Result) []string {
	var problems []string

	for _, name := range sortedKeys(caps) {
		subj, _ := parseCapture(caps[name]) // checked when the file was loaded
		v, err := subj.read(res)
		if err != nil {
			problems = append(problems, fmt.Sprintf("capture %s: %s", name, err))
			continue
		}
		sc.captured[name] = v
	}

	return problems
}

func isNumber(v json.RawMessage) bool {
	var n json.Number
	return json.Unmarshal(v, &n) == nil && len(v) > 0 && v[0] != '"'
}
