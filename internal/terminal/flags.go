package terminal

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Flags struct {
	Verbose        *bool
	PrintURL       *bool
	PrintTime      *bool
	PrintStatus    *bool
	PrintHeaders   *bool
	Pretty         *bool
	Fail           *bool
	ForceHttp      *bool // I don't remember for which usecase I added this flag.
	Raw            *bool
	Help           *bool
	Timeout        *int // -m, in seconds; -1 when it wasn't given
	RequestHeaders *[]string
}

func newFlag() Flags {
	// On a bad flag, the flag package prints the error, then this instead of its list of every flag.
	flag.Usage = func() {
		LogFC(TextDim, "zz -h shows the flags\n")
	}

	var headers []string
	flag.Func("H", `header "Key: Value" (repeatable)`, func(s string) error {
		if !strings.Contains(s, ":") {
			return fmt.Errorf("want \"Key: Value\", got %q", s)
		}
		headers = append(headers, s)
		return nil
	})

	timeout := -1
	flag.Func("m", "time limit in seconds, 0 for none (default: the config's timeout, 30)", func(s string) error {
		n, err := parseSeconds(s)
		if err != nil {
			return err
		}
		timeout = n
		return nil
	})

	var help bool
	flag.BoolVar(&help, "h", false, "show help")
	flag.BoolVar(&help, "help", false, "show help")

	f := Flags{
		Verbose:        flag.Bool("v", false, "show the config entry used, the url, time, status and response headers"),
		PrintURL:       flag.Bool("u", false, "show the url"),
		PrintTime:      flag.Bool("t", false, "show the time taken"),
		PrintStatus:    flag.Bool("s", false, "show the status"),
		PrintHeaders:   flag.Bool("i", false, "show the response headers"),
		Pretty:         flag.Bool("p", false, "pretty-print a JSON body"),
		Fail:           flag.Bool("f", false, "exit with status 22 on a 4xx or 5xx response (the body is still printed)"),
		ForceHttp:      flag.Bool("d", false, "use http:// for urls typed without a scheme"),
		Raw:            flag.Bool("r", false, "send the body exactly as typed: no fix-ups, no Content-Type detected"),
		Help:           &help,
		Timeout:        &timeout,
		RequestHeaders: &headers,
	}

	return f
}

// parseSeconds reads a whole number of seconds, 0 or more.
func parseSeconds(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0, fmt.Errorf("want a whole number of seconds, 0 or more; got %q", s)
	}
	return n, nil
}

type Args struct {
	Args  []string
	Flags Flags
}

func GetArgs() Args {
	f := newFlag()

	return Args{
		Flags: f,
		Args:  parseAnywhere(os.Args[1:]),
	}
}

// parseAnywhere parses the flags wherever they are, so `zz /users -v` works like
// `zz -v /users`, and returns the arguments that aren't flags, in order. flag.Parse stops
// at the first argument that isn't a flag, so it is run again on what follows each one.
// A flag's value (-H "Key: Value", -m 5) is taken by the flag package, never mistaken for
// an argument.
func parseAnywhere(args []string) []string {
	var positional []string

	for {
		_ = flag.CommandLine.Parse(args) // a bad flag exits with the usage (flag.ExitOnError)
		args = flag.Args()
		if len(args) == 0 {
			return positional
		}

		positional = append(positional, args[0])
		args = args[1:]
	}
}
