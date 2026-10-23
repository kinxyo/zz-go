package terminal

import (
	"flag"
	"fmt"
	"strings"
)

type Flags struct {
	Verbose      *bool
	PrintURL     *bool
	PrintTime    *bool
	PrintStatus  *bool
	PrintHeaders *bool
	Pretty       *bool
	ForceHttp    *bool
	Help         *bool
	Headers      *[]string
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

	var help bool
	flag.BoolVar(&help, "h", false, "show help")
	flag.BoolVar(&help, "help", false, "show help")

	f := Flags{
		Verbose:      flag.Bool("v", false, "show the url, time, status and response headers"),
		PrintURL:     flag.Bool("u", false, "show the url"),
		PrintTime:    flag.Bool("t", false, "show the time taken"),
		PrintStatus:  flag.Bool("s", false, "show the status"),
		PrintHeaders: flag.Bool("i", false, "show the response headers"),
		Pretty:       flag.Bool("p", false, "pretty-print a JSON body"),
		ForceHttp:    flag.Bool("d", false, "use http:// for urls typed without a scheme"),
		Help:         &help,
		Headers:      &headers,
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
