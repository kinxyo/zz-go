package collection

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/kinxyo/zz-go/internal/network"
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

func TestRead(t *testing.T) {
	res := &network.Result{
		StatusCode: 201,
		Header:     http.Header{"Location": {"/p/7"}, "X-Two": {"a", "b"}},
		Body: []byte(`{
			"id": 7,
			"name": "héllo",
			"ok": true,
			"none": null,
			"items": [{"id": "a"}, {"id": "b", "tags": []}],
			"obj": {"length": "mine"},
			"pair": {"a": 1, "b": 2}
		}`),
	}

	for in, want := range map[string]string{
		"status":                   `201`,
		"header Location":          `"/p/7"`,
		"header location":          `"/p/7"`,
		"header X-Two":             `"a, b"`,
		"json.id":                  `7`,
		"json.name":                `"héllo"`,
		"json.ok":                  `true`,
		"json.none":                `null`,
		"json.items.1.id":          `"b"`,
		"json.items.1":             `{"id":"b","tags":[]}`,
		"json.items.length":        `2`,
		"json.items.1.tags.length": `0`,
		"json.name.length":         `5`,
		"json.pair.length":         `2`,
		"json.obj.length":          `"mine"`,
		"json.length":              `7`,
	} {
		subj, _, err := parseSubject(in)
		if err != nil {
			t.Fatalf("parseSubject(%q): %v", in, err)
		}
		got, err := subj.read(res)
		if err != nil || string(got) != want {
			t.Errorf("read %q = %s, %v; want %s", in, got, err, want)
		}
	}

	for in, want := range map[string]string{
		"header Nope":         "header Nope not found",
		"json.nope":           "json.nope not found",
		"json.items.2":        "json.items.2 not found",
		"json.items.-1":       "json.items.-1 not found",
		"json.items.+1":       "json.items.+1 not found",
		"json.id.x":           "json.id.x not found",
		"json.items.length.x": "json.items.length.x not found",
		"json.id.length":      "json.id.length not found",
	} {
		subj, _, _ := parseSubject(in)
		if _, err := subj.read(res); err == nil || err.Error() != want {
			t.Errorf("read %q: %v; want %q", in, err, want)
		}
	}

	text := &network.Result{Body: []byte("created\n")}
	if got, err := (subject{kind: "text"}).read(text); err != nil || string(got) != `"created\n"` {
		t.Errorf("text = %s, %v", got, err)
	}
	for _, body := range []string{"created", ""} {
		_, err := (subject{kind: "json", path: []string{"id"}}).read(&network.Result{Body: []byte(body)})
		if err == nil || err.Error() != "response isn't JSON" {
			t.Errorf("json.id of %q: %v; want response isn't JSON", body, err)
		}
	}
}

func TestCapture(t *testing.T) {
	sc := (&Collection{}).scope()
	res := &network.Result{Header: http.Header{"Location": {"/p/7"}}, Body: []byte(`{"id": 7, "user": {"name": "a"}}`)}

	problems := sc.capture(map[string]string{
		"id":   "json.id",
		"user": "json.user",
		"loc":  "header Location",
		"gone": "json.gone",
	}, res)

	if want := []string{"capture gone: json.gone not found"}; !reflect.DeepEqual(problems, want) {
		t.Errorf("problems = %q; want %q", problems, want)
	}
	for name, want := range map[string]string{"id": `7`, "user": `{"name":"a"}`, "loc": `"/p/7"`} {
		if got := string(sc.captured[name]); got != want {
			t.Errorf("captured %s = %s; want %s, with its type", name, got, want)
		}
	}
	if _, ok := sc.captured["gone"]; ok {
		t.Error("a capture that found nothing has a value")
	}
}
