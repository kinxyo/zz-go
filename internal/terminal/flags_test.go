package terminal

import "testing"

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
