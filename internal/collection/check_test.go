package collection

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseExpect(t *testing.T) {
	cases := []struct {
		in   string
		want expectation
	}{
		{"status 201", expectation{subject{kind: "status"}, "==", []byte(`201`)}},
		{"status 2xx", expectation{subject{kind: "status"}, "==", []byte(`"2xx"`)}},
		{"status 4XX", expectation{subject{kind: "status"}, "==", []byte(`"4xx"`)}},
		{"text == created!", expectation{subject{kind: "text"}, "==", []byte(`"created!"`)}},
		{"text contains two words", expectation{subject{kind: "text"}, "contains", []byte(`"two words"`)}},
		{"header Content-Type contains json", expectation{subject{kind: "header", header: "Content-Type"}, "contains", []byte(`"json"`)}},
		{"json.success == true", expectation{subject{kind: "json", path: []string{"success"}}, "==", []byte(`true`)}},
		{`json.id == "12"`, expectation{subject{kind: "json", path: []string{"id"}}, "==", []byte(`"12"`)}},
		{"json.items.0.id exists", expectation{subject{kind: "json", path: []string{"items", "0", "id"}}, "exists", nil}},
		{"json.length > 0", expectation{subject{kind: "json", path: []string{"length"}}, ">", []byte(`0`)}},
		{"json.n <= 2.5", expectation{subject{kind: "json", path: []string{"n"}}, "<=", []byte(`2.5`)}},
		{"json.tags contains go", expectation{subject{kind: "json", path: []string{"tags"}}, "contains", []byte(`"go"`)}},
		{"json.x == null", expectation{subject{kind: "json", path: []string{"x"}}, "==", []byte(`null`)}},
		{"json != {}", expectation{subject{kind: "json"}, "!=", []byte(`{}`)}},
		{"  json.a   ==   padded  ", expectation{subject{kind: "json", path: []string{"a"}}, "==", []byte(`"padded"`)}},
	}

	for _, c := range cases {
		got, err := parseExpect(c.in)
		if err != nil {
			t.Errorf("parseExpect(%q): %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseExpect(%q) = %+v (value %s); want %+v (value %s)", c.in, got, got.value, c.want, c.want.value)
		}
	}
}

func TestParseExpectErrors(t *testing.T) {
	cases := map[string]string{
		"status 2x1":           "want a code like 201 or a class like 2xx",
		"status 600":           "want a code like 201 or a class like 2xx",
		"status":               "want a code like 201 or a class like 2xx",
		"text":                 "missing an operator",
		"text is hi":           `"is" isn't an operator`,
		"json.a exists 1":      `"exists" takes no value`,
		"json.a ==":            `"==" needs a value`,
		"json.n > abc":         "> compares numbers, and abc isn't one",
		`json.n < "1"`:         `< compares numbers, and "1" isn't one`,
		"json..a exists":       "a path is keys and indexes joined by dots",
		"json.a. exists":       "a path is keys and indexes joined by dots",
		"header":               `"header" needs a name`,
		"header Bad:Name == x": `"Bad:Name" isn't a valid header name`,
		"body == x":            `"body" isn't something zz can check`,
		"":                     `"" isn't something zz can check`,
	}

	for in, want := range cases {
		_, err := parseExpect(in)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("parseExpect(%q) = %v; want an error with %q", in, err, want)
		}
	}
}

func TestParseCapture(t *testing.T) {
	for in, want := range map[string]subject{
		"json.access_token": {kind: "json", path: []string{"access_token"}},
		"json.data.0.id":    {kind: "json", path: []string{"data", "0", "id"}},
		"json":              {kind: "json"},
		"header Location":   {kind: "header", header: "Location"},
	} {
		got, err := parseCapture(in)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("parseCapture(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}

	for _, bad := range []string{"status", "text", "json.id == 1", "header", "header Location extra", "cookie sid"} {
		if _, err := parseCapture(bad); err == nil {
			t.Errorf("parseCapture(%q): want an error", bad)
		}
	}
}
