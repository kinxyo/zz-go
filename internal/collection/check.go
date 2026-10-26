package collection

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
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

func isNumber(v json.RawMessage) bool {
	var n json.Number
	return json.Unmarshal(v, &n) == nil && len(v) > 0 && v[0] != '"'
}
