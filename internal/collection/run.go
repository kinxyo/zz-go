package collection

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kinxyo/zz-go/internal/config"
	"github.com/kinxyo/zz-go/internal/network"
	"github.com/kinxyo/zz-go/internal/terminal"
)

const exitInvalid = 2 // the file or an argument is wrong; nothing was sent

// Run runs `zz run <file>`. For now it loads and checks the collection, then prints each
// request as it would be sent. Nothing is sent yet.
func Run(args []string, flags *terminal.Flags) {
	if err := checkArgs(args, flags); err != nil {
		terminal.FatalWith(exitInvalid, "%s\n", err)
	}

	c, err := Load(args[1])
	if err != nil {
		terminal.FatalWith(exitInvalid, "%s\n", err)
	}

	// the collection's own project entry, wherever zz run is typed from
	dir, _ := filepath.Abs(filepath.Dir(c.path))
	conf, _, warnings := config.Load(dir)
	for _, w := range warnings {
		terminal.Err("config: %s\n", w)
	}

	c.printDry(conf, flags)
}

// checkArgs wants `run <file>`, and refuses the flags that mean nothing for a run instead of
// ignoring them.
func checkArgs(args []string, flags *terminal.Flags) error {
	if len(args) < 2 {
		return errors.New("zz run needs a collection file: zz run api.zz.json")
	}
	if len(args) > 2 {
		return fmt.Errorf("zz run takes one file; got %d: %s", len(args)-1, strings.Join(args[1:], " "))
	}

	for _, f := range []struct {
		name string
		set  bool
	}{
		{"-f", *flags.Fail},
		{"-u", *flags.PrintURL},
		{"-t", *flags.PrintTime},
		{"-s", *flags.PrintStatus},
		{"-i", *flags.PrintHeaders},
		{"-r", *flags.Raw},
	} {
		if f.set {
			return fmt.Errorf("%s doesn't apply to zz run (these do: -H, -m, -d, -v, -p)", f.name)
		}
	}

	return nil
}

// prepare builds a request as it will be sent: variables filled in from sc (one without a
// value yet stays as {{name}}), the URL made full, the body and its Content-Type decided, and
// the headers layered, lowest first: the collection's, auth's (not on login, nor with
// "auth": "none"), the request's, then -H. BuildHeaders lets later ones win.
func (c *Collection) prepare(r *Request, login bool, sc *scope, conf config.Settings, flags *terminal.Flags) (network.Request, error) {
	method, _ := network.MatchMethod(r.Method)
	nr := network.Request{Method: method, URL: sc.expand(r.URL)}
	nr.MakeURL(base(sc, conf, *flags.ForceHttp), *flags.ForceHttp)

	contentType := ""
	if body, ok := r.textBody(); ok {
		nr.Body = sc.expand(body)

		// decided once every value is in; until then (only when nothing is sent) it stays unset
		if names, _ := varNames(nr.Body); len(names) == 0 {
			ct, err := nr.ParseBody()
			if err != nil {
				return nr, err
			}
			contentType = ct
		}
	} else if len(r.Body) > 0 && string(r.Body) != "null" {
		nr.Body = string(sc.expandJSON(r.Body))
		contentType = "application/json"
	}

	var raw []string
	layer := func(h map[string]string) {
		for _, name := range sortedKeys(h) {
			raw = append(raw, name+": "+sc.expand(h[name]))
		}
	}
	layer(c.Headers)
	if c.Auth != nil && !login && r.Auth != "none" {
		layer(c.Auth.Headers)
	}
	layer(r.Headers)
	raw = append(raw, *flags.RequestHeaders...)

	if err := nr.BuildHeaders(contentType, raw, conf.UserAgent); err != nil {
		return nr, err
	}

	seconds := conf.Timeout
	if *flags.Timeout >= 0 { // -m wins when it was given
		seconds = *flags.Timeout
	}
	nr.Timeout = time.Duration(seconds) * time.Second

	return nr, nil
}

// base is where /path URLs go: the baseURL variable when there is one, made full like a URL
// typed on the CLI; otherwise the config's host and port.
func base(sc *scope, conf config.Settings, forceHTTP bool) string {
	v, ok := sc.lookup("baseURL")
	if !ok {
		return conf.BaseURL()
	}

	b := network.Request{URL: text(v)}
	b.MakeURL("", forceHTTP)
	return strings.TrimSuffix(b.URL, "/")
}

// printDry prints every request as it would be sent, login first. It is what zz run shows
// until requests are sent.
func (c *Collection) printDry(conf config.Settings, flags *terminal.Flags) {
	title := c.Name
	if title == "" {
		title = filepath.Base(c.path)
	}
	terminal.PrintFC(terminal.TextBold, "%s\n", title)
	terminal.PrintFC(terminal.TextDim, "checked; not sent: zz run only shows the requests for now. A {{name}} still in place is captured during the run.\n")

	sc := c.scope()

	if c.Auth != nil && c.Auth.Login != nil {
		c.printOne("login", c.Auth.Login, true, sc, conf, flags)
	}
	for i := range c.Requests {
		c.printOne(c.label(i), &c.Requests[i], false, sc, conf, flags)
	}
}

func (c *Collection) printOne(label string, r *Request, login bool, sc *scope, conf config.Settings, flags *terminal.Flags) {
	terminal.Print()
	terminal.PrintFC(terminal.TextBold, "%s\n", label)

	nr, err := c.prepare(r, login, sc, conf, flags)
	if err != nil {
		terminal.PrintFC(terminal.TextRed, "%s\n", err)
		return
	}

	terminal.PrintF("%s %s\n", nr.Method, nr.URL)

	names := make([]string, 0, len(nr.Header))
	for name := range nr.Header {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, v := range nr.Header[name] {
			terminal.PrintFC(terminal.TextDim, "%s: %s\n", name, v)
		}
	}

	if nr.Body != "" {
		terminal.Print()
		terminal.Print(nr.Body)
	}
}
