package cli

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/kinxyo/zz-go/internal/config"
	"github.com/kinxyo/zz-go/internal/network"
	"github.com/kinxyo/zz-go/internal/terminal"
)

// Run sends one request. project is the key of the config's project entry that conf came
// from, "" when there is none; -v shows it.
func Run(args []string, flags *terminal.Flags, conf config.Settings, project string) {

	// Extracting method, url, body into a Request object.
	r, err := extractIntoRequest(args, flags, conf)
	if err != nil {
		terminal.FatalWith(exitUsage, "%s\n", err)
	}

	// 1. Parse URL
	r.MakeURL(conf.BaseURL(), *flags.ForceHttp)

	// 2. Read Body from a file, for @path (even with -r: -r is about what's done to the body)
	if err := r.ReadBody(); err != nil {
		terminal.FatalWith(exitUsage, "%s\n", err)
	}

	// 3. Parse Body (-r sends it as typed, with no Content-Type detected)
	bodyType := ""
	if !*flags.Raw {
		bodyType, err = r.ParseBody()
		if err != nil {
			terminal.FatalWith(exitUsage, "%s\n", err)
		}
	}

	// 4. Parse Headers
	if err := r.BuildHeaders(bodyType, *flags.RequestHeaders, conf.UserAgent); err != nil {
		terminal.FatalWith(exitUsage, "%s\n", err)
	}

	// 5. Set Timeout
	seconds := conf.Timeout
	if *flags.Timeout >= 0 { // -m wins when it was given (-1 means it wasn't), otherwise the config's.
		seconds = *flags.Timeout
	}
	r.Timeout = time.Duration(seconds) * time.Second

	// 6. Make network request. -v first says which settings it was made with, so a request
	// that fails still shows it (a renamed directory no longer matches its project entry).
	if *flags.Verbose {
		if project == "" {
			terminal.LogFC(terminal.TextDim, "config: no project entry\n")
		} else {
			terminal.LogFC(terminal.TextDim, "config: project %s\n", project)
		}
	}

	res, err := r.CURL()
	if err != nil {
		terminal.FatalWith(exitCode(err), "%s\n", err)
	}

	// 7. Aftermath
	shown := res.Print(printOpts(flags, r.Method))

	if *flags.Fail && res.StatusCode >= 400 {
		terminal.ExitWith(exitHTTPError)
	}

	if !shown {
		terminal.ExitWith(exitOutputRefused)
	}
}

// printOpts decides what to show. -v turns on all the metadata. HEAD and OPTIONS
// replies have nothing but a status and headers, so those are always shown.
func printOpts(flags *terminal.Flags, method string) network.PrintOpts {
	v := *flags.Verbose
	bodyless := method == http.MethodHead || method == http.MethodOptions

	return network.PrintOpts{
		URL:     v || *flags.PrintURL,
		Time:    v || *flags.PrintTime,
		Status:  v || bodyless || *flags.PrintStatus,
		Headers: v || bodyless || *flags.PrintHeaders,
		Pretty:  *flags.Pretty,
	}
}

// extractIntoRequest puts `[method] <url> [body]` into a request, each as typed. It only
// places the arguments; Run's later steps parse them.
func extractIntoRequest(args []string, flags *terminal.Flags, settings config.Settings) (network.Request, error) {

	// TODO: query := parseHeaders(args)
	// TODO: params := parseHeaders(args)

	// 1. Extract Method & URL
	r := network.Request{Method: http.MethodGet}

	if m, ok := network.MatchMethod(args[0]); ok {
		// zz get <url>: with the method cut off, the url is always args[0]
		r.Method, args = m, args[1:]
	}

	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return r, errors.New("missing URL")
	}

	// zz [method] <url>
	r.URL = args[0]

	// 2. Extract Body
	if len(args) > 1 {
		// zz [method] <url> <body>
		r.Body = args[1]
	}

	return r, nil
}
