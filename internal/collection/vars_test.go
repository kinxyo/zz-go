package collection

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestLookupOrder(t *testing.T) {
	t.Setenv("ZZ_A", "from env")
	t.Setenv("ZZ_B", "from env")
	t.Setenv("ZZ_C", "from env")
	t.Setenv("ZZ_D", "from env")

	sc := &scope{
		captured:  map[string]json.RawMessage{"ZZ_A": json.RawMessage(`"captured"`)},
		variables: map[string]json.RawMessage{"ZZ_A": json.RawMessage(`"variables"`), "ZZ_B": json.RawMessage(`"variables"`)},
		dotenv:    map[string]string{"ZZ_A": "dotenv", "ZZ_B": "dotenv", "ZZ_C": "dotenv"},
	}

	for name, want := range map[string]string{"ZZ_A": "captured", "ZZ_B": "variables", "ZZ_C": "dotenv", "ZZ_D": "from env"} {
		v, ok := sc.lookup(name)
		if !ok || text(v) != want {
			t.Errorf("%s = %s, %v; want %q", name, v, ok, want)
		}
	}

	if _, ok := sc.lookup("ZZ_NOWHERE"); ok {
		t.Error("ZZ_NOWHERE has a value")
	}
	if sc.static("ZZ_NOWHERE") || !sc.static("ZZ_C") {
		t.Error("static is wrong")
	}

	sc.captured["ZZ_ONLY_CAPTURED"] = json.RawMessage(`1`)
	if sc.static("ZZ_ONLY_CAPTURED") {
		t.Error("a captured name isn't static")
	}
}

func testScope() *scope {
	return &scope{
		captured: map[string]json.RawMessage{
			"id":   json.RawMessage(`42`),
			"name": json.RawMessage(`"Al \"the\" <pal>"`),
			"obj":  json.RawMessage(`{"a":[1,2]}`),
			"flag": json.RawMessage(`true`),
		},
	}
}

func TestExpand(t *testing.T) {
	sc := testScope()

	cases := map[string]string{
		"/post/{{id}}":          "/post/42",
		"hi {{name}}":           `hi Al "the" <pal>`,
		"{{obj}}":               `{"a":[1,2]}`,
		"{{flag}}/{{id}}":       "true/42",
		"/x/{{later}}":          "/x/{{later}}", // no value yet: stays
		"no variables":          "no variables",
		"{{ id }} isn't a name": "{{ id }} isn't a name",
	}
	for in, want := range cases {
		if got := sc.expand(in); got != want {
			t.Errorf("expand(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestExpandJSON(t *testing.T) {
	sc := testScope()

	cases := []struct {
		name, in, want string
	}{
		{"key order is kept", `{"z":1,"a":"{{id}}","m":2}`, `{"z":1,"a":42,"m":2}`},
		{"a string that is only a variable takes its type", `{"id":"{{id}}","ok":"{{flag}}","o":"{{obj}}"}`, `{"id":42,"ok":true,"o":{"a":[1,2]}}`},
		{"a string value stays a string", `{"n":"{{name}}"}`, `{"n":"Al \"the\" <pal>"}`},
		{"inside a longer string, it's text, escaped", `{"s":"by {{name}} #{{id}}"}`, `{"s":"by Al \"the\" <pal> #42"}`},
		{"a key gets text, never a type", `{"{{id}}":"{{id}}"}`, `{"42":42}`},
		{"in an array", `["{{id}}","x{{id}}"]`, `[42,"x42"]`},
		{"no value yet: left as written", `{"a":"{{later}}","b":"x {{later}}"}`, `{"a":"{{later}}","b":"x {{later}}"}`},
		{"escaped quotes don't end a string", `{"q":"say \"{{id}}\"","n":1.50}`, `{"q":"say \"42\"","n":1.50}`},
		{"numbers are kept as written", `{"n":1.0,"e":1e3}`, `{"n":1.0,"e":1e3}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(sc.expandJSON([]byte(c.in)))
			if got != c.want {
				t.Errorf("got  %s\nwant %s", got, c.want)
			}
			if !json.Valid([]byte(got)) {
				t.Errorf("not valid JSON: %s", got)
			}
		})
	}
}

func TestVarNames(t *testing.T) {
	names, err := varNames("/a/{{x}}/{{y.z}}?q={{x}}&{not}")
	if err != nil || !reflect.DeepEqual(names, []string{"x", "y.z", "x"}) {
		t.Errorf("got %v, %v", names, err)
	}

	for _, bad := range []string{"{{ x }}", "{{}}", "{{a b}}", "{{$random}}"} {
		if _, err := varNames(bad); err == nil {
			t.Errorf("varNames(%q): want an error", bad)
		}
	}
}

func TestParseDotenv(t *testing.T) {
	env, err := parseDotenv(strings.Join([]string{
		"# a comment",
		"",
		"PLAIN=value",
		"  SPACED = padded  ",
		"export EXPORTED=yes",
		`DOUBLE="a # not a comment\nnext \"q\" \\"`,
		`SINGLE='kept \n as # is'`,
		"COMMENTED=value # a comment",
		"EMPTY=",
		"EQUALS=a=b=c",
		"URL=http://x.io/#frag",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"PLAIN":     "value",
		"SPACED":    "padded",
		"EXPORTED":  "yes",
		"DOUBLE":    "a # not a comment\nnext \"q\" \\",
		"SINGLE":    `kept \n as # is`,
		"COMMENTED": "value",
		"EMPTY":     "",
		"EQUALS":    "a=b=c",
		"URL":       "http://x.io/#frag",
	}
	if !reflect.DeepEqual(env, want) {
		t.Errorf("got  %q\nwant %q", env, want)
	}
}

func TestParseDotenvErrors(t *testing.T) {
	for _, bad := range []string{"OK=1\nno equals sign", "=value"} {
		_, err := parseDotenv(bad)
		if err == nil || !strings.Contains(err.Error(), "want KEY=VALUE") {
			t.Errorf("parseDotenv(%q) = %v", bad, err)
		}
	}

	_, err := parseDotenv("A=1\n\nsecret-without-equals")
	if err == nil || err.Error() != "3: want KEY=VALUE" {
		t.Errorf("the error gives the line number and never the line: %v", err)
	}
}
