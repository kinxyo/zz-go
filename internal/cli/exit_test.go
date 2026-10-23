package cli

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/kinxyo/zz-go/internal/network"
)

func TestExitCode(t *testing.T) {
	cases := []struct {
		kind error
		want int
	}{
		{network.ErrInvalidRequest, 2},
		{network.ErrNoSuchHost, 6},
		{network.ErrRefused, 7},
		{network.ErrTimeout, 28},
		{network.ErrPlainHTTP, 35},
		{network.ErrCertificate, 60},
	}

	for _, c := range cases {
		// wrapped twice, the way a real error arrives
		err := fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", c.kind))
		if got := exitCode(err); got != c.want {
			t.Errorf("exitCode(%v) = %d; want %d", c.kind, got, c.want)
		}
	}

	if got := exitCode(errors.New("something else")); got != 1 {
		t.Errorf("an unknown error exits %d; want 1", got)
	}
}

func TestExitCodesTable(t *testing.T) {
	seen := map[int]bool{}
	for i, e := range ExitCodes {
		if seen[e.Code] {
			t.Errorf("code %d is listed twice", e.Code)
		}
		seen[e.Code] = true

		if i > 0 && e.Code < ExitCodes[i-1].Code {
			t.Errorf("code %d comes after %d; keep the list in order", e.Code, ExitCodes[i-1].Code)
		}
		if strings.TrimSpace(e.Meaning) == "" {
			t.Errorf("code %d has no meaning", e.Code)
		}
	}

	// every code the program can exit with is in the table
	for _, code := range []int{exitOK, exitFailure, exitUsage, exitNoSuchHost, exitRefused,
		exitHTTPError, exitOutputRefused, exitTimeout, exitPlainHTTP, exitCertificate} {
		if !seen[code] {
			t.Errorf("exit code %d is used but not in ExitCodes", code)
		}
	}
}

// TestReadmeListsEveryExitCode keeps the README's table in step with ExitCodes.
func TestReadmeListsEveryExitCode(t *testing.T) {
	data, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}

	section := string(data)
	start := strings.Index(section, "### Exit codes")
	if start < 0 {
		t.Fatal(`README has no "### Exit codes" section`)
	}
	section = section[start:]
	if end := strings.Index(section[1:], "\n#"); end >= 0 {
		section = section[:end+1]
	}

	var readme []int
	for _, m := range regexp.MustCompile(`(?m)^\| (\d+) \|`).FindAllStringSubmatch(section, -1) {
		code, _ := strconv.Atoi(m[1])
		readme = append(readme, code)
	}

	var table []int
	for _, e := range ExitCodes {
		table = append(table, e.Code)
	}
	sort.Ints(readme)

	if fmt.Sprint(readme) != fmt.Sprint(table) {
		t.Errorf("README lists exit codes %v; ExitCodes has %v", readme, table)
	}
}
