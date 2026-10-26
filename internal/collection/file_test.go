package collection

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// write puts a file in a temp dir and returns its path.
func write(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func load(t *testing.T, content string) *Collection {
	t.Helper()

	c, err := Load(write(t, "api.zz.json", content))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return c
}

func TestLoadReadsTheShape(t *testing.T) {
	c := load(t, `{
		"zz": 1,
		"name": "posts api",
		"variables": {"baseURL": "http://localhost:8000", "n": 10, "cfg": { "a" : [1, 2] }},
		"headers": {"X-API-Version": "2.0"},
		"auth": {
			"login": {"method": "post", "url": "/login", "body": {"u": "a"}, "capture": {"token": "json.token"}},
			"headers": {"Authorization": "Bearer {{token}}"}
		},
		"requests": [
			{"name": "create", "method": "POST", "url": "/post", "body": {
				"content": "hi",
				"tags": [ "a", "b" ]
			}, "expect": ["status 201"], "capture": {"id": "json.id"}},
			{"method": "GET", "url": "/post/{{id}}", "auth": "none", "headers": {"Accept": "application/json"}},
			{"method": "POST", "url": "/note", "body": "{text: hi}"}
		]
	}`)

	if c.Version != 1 || c.Name != "posts api" || len(c.Requests) != 3 {
		t.Fatalf("got %+v", c)
	}
	if got := string(c.Variables["cfg"]); got != `{"a":[1,2]}` {
		t.Errorf("a variable's value is compacted: got %s", got)
	}
	if got := string(c.Requests[0].Body); got != `{"content":"hi","tags":["a","b"]}` {
		t.Errorf("a JSON body is compacted, in its order: got %s", got)
	}
	if got := string(c.Auth.Login.Body); got != `{"u":"a"}` {
		t.Errorf("login body = %s", got)
	}
	if body, ok := c.Requests[2].textBody(); !ok || body != "{text: hi}" {
		t.Errorf("a string body = %q, %v", body, ok)
	}
	if c.Requests[1].Auth != "none" || c.Requests[1].Headers["Accept"] != "application/json" {
		t.Errorf("request 2 = %+v", c.Requests[1])
	}
	if !reflect.DeepEqual(c.Auth.Login.Capture, map[string]string{"token": "json.token"}) {
		t.Errorf("login capture = %v", c.Auth.Login.Capture)
	}
}

func TestLoadTheExample(t *testing.T) {
	if _, err := Load("../../example/api.zz.json"); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDotenv(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte("EMAIL=a@b.c\n"), 0o644)
	path := filepath.Join(dir, "api.zz.json")
	os.WriteFile(path, []byte(`{"zz": 1, "dotenv": ".env", "requests": [{"method": "GET", "url": "/{{EMAIL}}"}]}`), 0o644)

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.env["EMAIL"] != "a@b.c" {
		t.Errorf("env = %v; the dotenv path is relative to the collection", c.env)
	}
}

func TestLoadErrors(t *testing.T) {
	t.Setenv("ZZ_TEST_FROM_ENV", "x")

	cases := []struct {
		name    string
		content string
		want    string // the error, after "api.zz.json"; each line must be in it
	}{
		{"syntax error with line and column", "{\n  \"zz\": 1,\n  \"requests\": [\n    {\"method\": \"GET\",}\n  ]\n}",
			`:4:22: invalid character '}' looking for beginning of object key string`},
		{"empty file", ``, `:1:1: unexpected end of JSON input`},
		{"a wrong type", `{"zz": 1, "requests": [{"method": "GET", "url": "/", "expect": "status 200"}]}`,
			`:1:75: requests.0.expect: want an array of strings, got a JSON string`},
		{"unknown field", `{"zz": 1, "requests": [{"method": "GET", "url": "/", "expct": []}]}`, `: unknown field "expct"`},
		{"not a collection", `{"name": "x", "dependencies": {}}`, `: no "zz" field; a collection starts with "zz": 1`},
		{"no zz", `{"requests": [{"method": "GET", "url": "/"}]}`, `: no "zz" field; a collection starts with "zz": 1`},
		{"another version", `{"zz": 2, "requests": []}`, `: "zz": 2 isn't a version this zz reads; it reads 1`},
		{"no requests", `{"zz": 1}`, `: no requests: "requests" is the list of requests to run`},
		{"missing dotenv", `{"zz": 1, "dotenv": "nope.env", "requests": [{"method": "GET", "url": "/"}]}`,
			`: dotenv: open `},
		{"bad variable name", `{"zz": 1, "variables": {"a b": 1}, "requests": [{"method": "GET", "url": "/"}]}`,
			`: variables: "a b" isn't a valid name: use letters, digits, _ - and .`},
		{"baseURL that isn't a URL string", `{"zz": 1, "variables": {"baseURL": 8000}, "requests": [{"method": "GET", "url": "/"}]}`,
			`: baseURL: want a URL like "http://localhost:8000", got 8000`},
		{"missing method", `{"zz": 1, "requests": [{"url": "/"}]}`, `: request 1: "method" is missing`},
		{"unknown method", `{"zz": 1, "requests": [{"method": "GRAB", "url": "/"}]}`, `: request 1: method "GRAB" isn't one zz knows`},
		{"missing url", `{"zz": 1, "requests": [{"name": "x", "method": "GET"}]}`, `: request 1 (x): "url" is missing`},
		{"bad header name", `{"zz": 1, "requests": [{"method": "GET", "url": "/", "headers": {"X Y": "1"}}]}`,
			`: request 1: headers: "X Y" isn't a valid header name`},
		{"same header twice", `{"zz": 1, "headers": {"Accept": "a", "accept": "b"}, "requests": [{"method": "GET", "url": "/"}]}`,
			`: headers: "Accept" and "accept" are the same header`},
		{"a line break in a header", `{"zz": 1, "requests": [{"method": "GET", "url": "/", "headers": {"X": "a\nb"}}]}`,
			`: request 1: headers: the value of "X" has a line break`},
		{"bad auth value", `{"zz": 1, "requests": [{"method": "GET", "url": "/", "auth": "admin"}]}`,
			`: request 1: "auth": "admin": the only value is "none" (no auth headers on this request)`},
		{"auth on login", `{"zz": 1, "auth": {"login": {"method": "POST", "url": "/l", "auth": "none"}}, "requests": [{"method": "GET", "url": "/"}]}`,
			`: auth.login: "auth" doesn't apply to login`},
		{"a bad login", `{"zz": 1, "auth": {"login": {"url": "/l"}}, "requests": [{"method": "GET", "url": "/"}]}`,
			`: auth.login: "method" is missing`},
		{"a malformed variable", `{"zz": 1, "requests": [{"method": "GET", "url": "/{{ id }}"}]}`,
			`: request 1: "{{ id }}" isn't a variable: a name is letters, digits, _ - and .`},
		{"undefined variable", `{"zz": 1, "requests": [{"method": "GET", "url": "/{{id}}"}]}`,
			`: request 1: {{id}} isn't defined: it's not in variables, the .env or the environment, and nothing above captures it`},
		{"used before it's captured", `{"zz": 1, "requests": [
			{"name": "get", "method": "GET", "url": "/{{id}}"},
			{"name": "create", "method": "POST", "url": "/", "capture": {"id": "json.id"}}]}`,
			`: request 1 (get): {{id}} is used before request 2 (create) captures it`},
		{"used by the request that captures it", `{"zz": 1, "requests": [{"method": "GET", "url": "/{{id}}", "capture": {"id": "json.id"}}]}`,
			`: request 1: {{id}} is used in the request that captures it`},
		{"collection headers can't use captures", `{"zz": 1, "headers": {"X": "{{id}}"}, "requests": [{"method": "GET", "url": "/", "capture": {"id": "json.id"}}]}`,
			`: headers: {{id}} isn't defined: it's not in variables, the .env or the environment (headers are sent on login too, before anything is captured)`},
		{"login can't use its own capture", `{"zz": 1, "auth": {"login": {"method": "POST", "url": "/{{token}}", "capture": {"token": "json.t"}}}, "requests": [{"method": "GET", "url": "/"}]}`,
			`: auth.login: {{token}} isn't defined`},
		{"auth headers only see login's captures", `{"zz": 1, "auth": {"headers": {"Authorization": "{{token}}"}}, "requests": [{"method": "GET", "url": "/", "capture": {"token": "json.t"}}]}`,
			`: auth.headers: {{token}} isn't defined: it's not in variables, the .env or the environment, and login doesn't capture it`},
		{"a string body that isn't JSON", `{"zz": 1, "requests": [{"method": "POST", "url": "/", "body": "{content: }"}]}`,
			`: request 1: body is not valid JSON`},
		{"bad expect", `{"zz": 1, "requests": [{"name": "c", "method": "GET", "url": "/", "expect": ["status 2x1"]}]}`,
			`: request 1 (c): expect "status 2x1": want a code like 201 or a class like 2xx`},
		{"bad capture subject", `{"zz": 1, "requests": [{"method": "GET", "url": "/", "capture": {"id": "status"}}]}`,
			`: request 1: capture "id": a capture is json.<path> or header <Name>`},
		{"bad capture name", `{"zz": 1, "requests": [{"method": "GET", "url": "/", "capture": {"a b": "json.id"}}]}`,
			`: request 1: capture "a b": not a valid variable name: use letters, digits, _ - and .`},
		{"every problem is listed", `{"zz": 1, "requests": [{"method": "GRAB", "url": "/"}, {"method": "GET"}]}`,
			": request 1: method \"GRAB\" isn't one zz knows\napi.zz.json: request 2: \"url\" is missing"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := write(t, "api.zz.json", c.content)

			_, err := Load(path)
			if err == nil {
				t.Fatal("want an error, got none")
			}
			got := strings.ReplaceAll(err.Error(), path, "api.zz.json")
			if !strings.HasPrefix(got, "api.zz.json") || !strings.Contains(got, c.want) {
				t.Errorf("error:\n%s\nwant it to contain:\napi.zz.json%s", got, c.want)
			}
		})
	}
}

func TestLoadFindsVariablesInEveryPlace(t *testing.T) {
	t.Setenv("ZZ_TEST_FROM_ENV", "x")

	// each name is defined one way: variables, the environment, captured by login, captured above
	load(t, `{
		"zz": 1,
		"variables": {"v": 1},
		"headers": {"X-A": "{{v}} {{ZZ_TEST_FROM_ENV}}"},
		"auth": {
			"login": {"method": "POST", "url": "/l/{{v}}", "body": {"p": "{{ZZ_TEST_FROM_ENV}}"}, "capture": {"token": "json.t"}},
			"headers": {"Authorization": "Bearer {{token}}"}
		},
		"requests": [
			{"method": "POST", "url": "/{{token}}", "capture": {"id": "json.id"}},
			{"method": "GET", "url": "/{{id}}", "headers": {"X-B": "{{id}}"}, "body": {"id": "{{id}}"}}
		]
	}`)
}
