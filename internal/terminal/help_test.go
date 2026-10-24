package terminal_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/kinxyo/zz-go/internal/cli"
	"github.com/kinxyo/zz-go/internal/terminal"
)

// TestHelpListsEveryExitCode keeps the help's EXIT summary in step with cli.ExitCodes.
func TestHelpListsEveryExitCode(t *testing.T) {
	exit := terminal.HelpText("EXIT")
	if exit == "" {
		t.Fatal("help has no EXIT section")
	}

	for _, e := range cli.ExitCodes {
		// the number on its own, so 2 doesn't match inside 22
		if !regexp.MustCompile(`(^|\D)` + strconv.Itoa(e.Code) + `(\D|$)`).MatchString(exit) {
			t.Errorf("help EXIT section doesn't list %d (%s)", e.Code, e.Meaning)
		}
	}

	for _, line := range strings.Split(strings.TrimSpace(exit), "\n") {
		if len(line)+2 > 80 {
			t.Errorf("help line is %d columns, wider than 80: %q", len(line)+2, line)
		}
	}
}
