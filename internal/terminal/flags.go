package terminal

import (
	"flag"
	"fmt"
	"strings"
)

type Flags struct {
	Verbose     *bool
	PrintURL    *bool
	PrintTime   *bool
	PrintStatus *bool
	NoFormat    *bool
	ForceSeq    *bool
	ForceHttp   *bool
	Headers     *[]string
}

func newFlag() Flags {
	var headers []string
	flag.Func("H", `header "Key: Value" (repeatable)`, func(s string) error {
		if !strings.Contains(s, ":") {
			return fmt.Errorf("want \"Key: Value\", got %q", s)
		}
		headers = append(headers, s)
		return nil
	})

	f := Flags{
		Verbose:     flag.Bool("v", false, "logging all meta info to terminal"),
		PrintURL:    flag.Bool("u", false, "log url to terminal"),
		PrintTime:   flag.Bool("t", false, "log time to terminal"),
		PrintStatus: flag.Bool("s", false, "log status to terminal"),
		NoFormat:    flag.Bool("n", false, "No pretty json"),
		ForceSeq:    flag.Bool("q", false, "force to run sequentially"),
		ForceHttp:   flag.Bool("h", false, "force http"),
		Headers:     &headers,
	}

	return f
}

type Args struct {
	Args  []string
	Flags Flags
}

func GetArgs() Args {
	f := newFlag()

	flag.Parse()

	return Args{
		Flags: f,
		Args:  flag.Args(),
	}
}
