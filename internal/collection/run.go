package collection

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"path/filepath"
	"strings"

	"github.com/kinxyo/zz-go/internal/config"
	"github.com/kinxyo/zz-go/internal/network"
	"github.com/kinxyo/zz-go/internal/terminal"
)

// Exit codes of zz run.
const (
	exitFailed  = 1 // login failed, or a request failed or was skipped
	exitInvalid = 2 // the file or an argument is wrong; nothing was sent
)

// Run runs `zz run <file>`: the whole file is checked first, then login and each request are
// sent in order, one line each, and it exits 0 when all of them passed.
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
	conf, project, warnings := config.Load(dir)
	for _, w := range warnings {
		terminal.Err("config: %s\n", w)
	}

	if code := c.run(conf, project, flags); code != 0 {
		terminal.ExitWith(code)
	}
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

	if *flags.Pretty && !*flags.Verbose {
		return errors.New("-p pretty-prints the bodies -v shows; add -v")
	}

	// once here, instead of failing every request with the same error
	if err := (&network.Request{}).BuildHeaders("", *flags.RequestHeaders, ""); err != nil {
		return err
	}

	return nil
}

// run sends login, then each request in order, and returns the exit code. A request that can't
// be sent, or whose capture finds nothing, fails, and the run goes on; one that needs a value
// that wasn't captured is skipped. A failed login stops the run.
func (c *Collection) run(conf config.Settings, project string, flags *terminal.Flags) int {
	title := c.Name
	if title == "" {
		title = filepath.Base(c.path)
	}
	terminal.PrintFC(terminal.TextBold, "%s\n", title)

	if *flags.Verbose {
		if project == "" {
			terminal.PrintFC(terminal.TextDim, "config: no project entry\n")
		} else {
			terminal.PrintFC(terminal.TextDim, "config: project %s\n", project)
		}
	}
	if c.hasExpect() {
		terminal.PrintFC(terminal.TextDim, "expect isn't checked yet: a request passes on any response, login on a 2xx\n")
	}
	terminal.Print()

	rp := c.newReport(*flags.Verbose, *flags.Pretty)
	sc := c.scope()
	jar, _ := cookiejar.New(nil) // never fails without options

	if c.Auth != nil && c.Auth.Login != nil {
		l := c.send("login", c.Auth.Login, true, sc, jar, conf, flags)
		if l.res != nil && l.res.StatusCode/100 != 2 {
			l.outcome = failed
			l.problems = append([]string{"login needs a 2xx status; got " + l.res.Status}, l.problems...)
		}
		rp.print(l)

		if l.outcome != passed {
			rp.stopped(len(c.Requests))
			return exitFailed
		}
	}

	for i := range c.Requests {
		r := &c.Requests[i]

		if name, from := c.missing(i, sc); name != "" {
			method, _ := network.MatchMethod(r.Method)
			note := fmt.Sprintf("skipped: needs {{%s}}", name)
			if from >= 0 {
				note += " from " + c.short(from)
			}
			rp.print(line{name: r.Name, method: method, url: r.URL, outcome: skipped, note: note})
			continue
		}

		rp.print(c.send(r.Name, r, false, sc, jar, conf, flags))
	}

	rp.summary()

	if rp.counts[failed]+rp.counts[skipped] > 0 {
		return exitFailed
	}
	return 0
}

// send builds and sends one request, then takes its captures into sc. It passes when a response
// came back and every capture found its value.
func (c *Collection) send(name string, r *Request, login bool, sc *scope, jar http.CookieJar, conf config.Settings, flags *terminal.Flags) line {
	method, _ := network.MatchMethod(r.Method)
	l := line{name: name, method: method, url: r.URL, outcome: failed}

	nr, err := c.prepare(r, login, sc, conf, flags)
	if err != nil {
		l.note = err.Error()
		return l
	}
	l.sent = &nr

	nr.Jar = jar
	res, err := nr.CURL()
	if err != nil {
		l.note = err.Error()
		return l
	}
	l.res = res

	l.problems = sc.capture(r.Capture, res)
	if len(l.problems) == 0 {
		l.outcome = passed
	}
	return l
}

// missing returns the first variable request i uses that has no value, and the nearest request
// above that captures it (-1 if none does). The file was checked before the run, so only a
// capture that found nothing, or a request that wasn't sent, leaves one.
func (c *Collection) missing(i int, sc *scope) (string, int) {
	for _, name := range c.Requests[i].used() {
		if _, ok := sc.lookup(name); ok {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			if _, ok := c.Requests[j].Capture[name]; ok {
				return name, j
			}
		}
		return name, -1
	}
	return "", -1
}

// short names request i in the report: by its name, or by position when it has none.
func (c *Collection) short(i int) string {
	if name := c.Requests[i].Name; name != "" {
		return name
	}
	return fmt.Sprintf("request %d", i+1)
}

// hasExpect reports whether login or any request has an expect, which isn't checked yet.
func (c *Collection) hasExpect() bool {
	if c.Auth != nil && c.Auth.Login != nil && len(c.Auth.Login.Expect) > 0 {
		return true
	}
	for _, r := range c.Requests {
		if len(r.Expect) > 0 {
			return true
		}
	}
	return false
}

// prepare builds a request as it will be sent: variables filled in from sc, the body and its
// Content-Type decided, and the rest done by network's Prepare, the same as for zz <url>. The
// headers are layered, lowest first: the collection's, auth's (not on login, nor with
// "auth": "none"), the request's, then -H. BuildHeaders lets later ones win.
func (c *Collection) prepare(r *Request, login bool, sc *scope, conf config.Settings, flags *terminal.Flags) (network.Request, error) {
	method, _ := network.MatchMethod(r.Method)
	nr := network.Request{Method: method, URL: sc.expand(r.URL)}

	contentType := ""
	if body, ok := r.textBody(); ok {
		nr.Body = sc.expand(body)
		ct, err := nr.ParseBody()
		if err != nil {
			return nr, err
		}
		contentType = ct
	} else if len(r.Body) > 0 && string(r.Body) != "null" {
		nr.Body = string(sc.expandJSON(r.Body))
		contentType = "application/json"
	}

	var layers []string
	layer := func(h map[string]string) {
		for _, name := range sortedKeys(h) {
			layers = append(layers, name+": "+sc.expand(h[name]))
		}
	}
	layer(c.Headers)
	if c.Auth != nil && !login && r.Auth != "none" {
		layer(c.Auth.Headers)
	}
	layer(r.Headers)

	err := nr.Prepare(conf, flags, network.Parts{
		Base:        base(sc, conf, *flags.ForceHttp),
		ContentType: contentType,
		Headers:     layers,
	})
	return nr, err
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
