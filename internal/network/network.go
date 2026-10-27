package network

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/kinxyo/zz-go/internal/config"
	"github.com/kinxyo/zz-go/internal/terminal"
)

// Request is what CURL sends.
type Request struct {
	Method  string
	URL     string
	Header  http.Header
	Body    string
	Timeout time.Duration  // covers the whole request, including reading the body; 0 means no limit
	Jar     http.CookieJar // cookies kept from one request to the next (zz run); nil keeps none
}

type Result struct {
	Method     string
	URL        string
	Proto      string // "HTTP/1.1"
	Status     string // "200 OK"
	StatusCode int
	Header     http.Header
	Body       []byte
	Elapsed    time.Duration
}

// Parts are what the caller decided about a request before Prepare finishes it. The rest comes
// from the config and the flags, the same way for zz <url> and zz run.
type Parts struct {
	Base        string   // where a /path URL goes
	ContentType string   // the body's, already decided; "" when there's none (no body, or -r)
	Headers     []string // "Key: Value", lowest priority first; -H goes on top of them
}

// Prepare finishes a request in the same steps for both modes: the URL made full, the headers
// layered (zz's defaults, p.Headers, then -H; see BuildHeaders), and the time limit. The body
// is set before: deciding it is where the two modes differ.
func (r *Request) Prepare(conf config.Settings, flags *terminal.Flags, p Parts) error {
	// 1. URL
	r.MakeURL(p.Base, *flags.ForceHttp)

	// 2. Headers
	if err := r.BuildHeaders(p.ContentType, slices.Concat(p.Headers, *flags.RequestHeaders), conf.UserAgent); err != nil {
		return err
	}

	// 3. Timeout: -m wins when it was given (-1 means it wasn't), otherwise the config's.
	seconds := conf.Timeout
	if *flags.Timeout >= 0 {
		seconds = *flags.Timeout
	}
	r.Timeout = time.Duration(seconds) * time.Second

	return nil
}

func (r *Request) CURL() (*Result, error) {

	start := time.Now() // -- STARTS
	// +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
	p := strings.NewReader(r.Body)
	req, err := http.NewRequest(r.Method, r.URL, p)
	if err != nil {
		// a malformed URL: the user's input, not the network
		return nil, &friendlyError{msg: err.Error(), kind: ErrInvalidRequest, err: err}
	}

	for k, vv := range r.Header {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}

	resp, err := (&http.Client{Timeout: r.Timeout, Jar: r.Jar}).Do(req)
	if err != nil {
		return nil, explain(err, req.URL.Host, r.Timeout)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, explain(err, req.URL.Host, r.Timeout)
	}

	// +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
	elapsed := time.Since(start) // -- ENDS

	return &Result{
		Method:     r.Method,
		URL:        r.URL,
		Proto:      resp.Proto,
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       body,
		Elapsed:    elapsed,
	}, nil
}
