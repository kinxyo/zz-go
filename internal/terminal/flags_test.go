package terminal

import (
	"flag"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestParseSeconds(t *testing.T) {
	good := map[string]int{"0": 0, "1": 1, "30": 30, " 5 ": 5, "3600": 3600}
	for in, want := range good {
		got, err := parseSeconds(in)
		if err != nil || got != want {
			t.Errorf("parseSeconds(%q) = %d, %v; want %d", in, got, err, want)
		}
	}

	for _, in := range []string{"-1", "1.5", "30s", "", "ten"} {
		if _, err := parseSeconds(in); err == nil {
			t.Errorf("parseSeconds(%q) should fail", in)
		}
	}
}

// parse runs parseAnywhere on a fresh flag set, so tests don't share flag values, and
// returns the flags and the positional arguments.
func parse(t *testing.T, args ...string) (Flags, []string) {
	t.Helper()

	savedSet, savedUsage := flag.CommandLine, flag.Usage
	t.Cleanup(func() { flag.CommandLine, flag.Usage = savedSet, savedUsage })

	flag.CommandLine = flag.NewFlagSet("zz", flag.ContinueOnError)
	f := newFlag()
	return f, parseAnywhere(args)
}

func TestParseAnywhere(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		positional []string
		verbose    bool
		headers    []string
		timeout    int
	}{
		{"no flags", []string{"p", "/x", "{a: 1}"}, []string{"p", "/x", "{a: 1}"}, false, nil, -1},
		{"flags first", []string{"-v", "p", "/x"}, []string{"p", "/x"}, true, nil, -1},
		{"flags last", []string{"p", "/x", "-v"}, []string{"p", "/x"}, true, nil, -1},
		{"flags in between", []string{"p", "-v", "/x", "{a: 1}"}, []string{"p", "/x", "{a: 1}"}, true, nil, -1},
		{"a flag's value isn't an argument", []string{"p", "/x", "-H", "X-A: 1", "{a: 1}", "-m", "5"},
			[]string{"p", "/x", "{a: 1}"}, false, []string{"X-A: 1"}, 5},
		{"-H in several places adds up", []string{"-H", "A: 1", "/x", "-H", "B: 2"},
			[]string{"/x"}, false, []string{"A: 1", "B: 2"}, -1},
		{"the last -m wins", []string{"-m", "5", "/x", "-m", "9"}, []string{"/x"}, false, nil, 9},
		{"- alone is an argument", []string{"p", "/x", "-"}, []string{"p", "/x", "-"}, false, nil, -1},
		{"nothing", nil, nil, false, nil, -1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, got := parse(t, c.args...)

			if !slices.Equal(got, c.positional) {
				t.Errorf("positional = %q; want %q", got, c.positional)
			}
			if *f.Verbose != c.verbose {
				t.Errorf("-v = %v; want %v", *f.Verbose, c.verbose)
			}
			if !slices.Equal(*f.RequestHeaders, c.headers) {
				t.Errorf("-H = %q; want %q", *f.RequestHeaders, c.headers)
			}
			if *f.Timeout != c.timeout {
				t.Errorf("-m = %d; want %d", *f.Timeout, c.timeout)
			}
		})
	}
}

// TestDocsListTheRealFlags keeps the help's FLAGS section and the README's flag table in step
// with the flags zz defines.
func TestDocsListTheRealFlags(t *testing.T) {
	parse(t) // registers zz's flags on a fresh flag set
	defined := map[string]bool{}
	flag.VisitAll(func(f *flag.Flag) { defined[f.Name] = true })

	inHelp := map[string]bool{}
	for _, s := range helpSections {
		if s.title != "FLAGS" {
			continue
		}
		for _, r := range s.rows {
			for _, m := range regexp.MustCompile(`(?:^|\s)-([A-Za-z])\b`).FindAllStringSubmatch(r.left, -1) {
				inHelp[m[1]] = true
			}
		}
	}
	inHelp["h"] = strings.Contains(HelpText("COMMANDS"), "-h")

	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	inReadme := map[string]bool{}
	for _, m := range regexp.MustCompile("(?m)^\\| `-([A-Za-z])").FindAllStringSubmatch(string(readme), -1) {
		inReadme[m[1]] = true
	}

	for name := range inHelp {
		if !defined[name] {
			t.Errorf("help lists -%s, which isn't a flag", name)
		}
	}
	for name := range inReadme {
		if !defined[name] {
			t.Errorf("README lists -%s, which isn't a flag", name)
		}
	}
	for name := range defined {
		if name == "help" { // the long form of -h
			continue
		}
		if !inHelp[name] {
			t.Errorf("-%s is a flag but the help doesn't list it", name)
		}
		if name != "h" && !inReadme[name] {
			t.Errorf("-%s is a flag but the README's flag table doesn't list it", name)
		}
	}
}
