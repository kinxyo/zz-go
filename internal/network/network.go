package network

import (
	"io"
	"net/http"
	"strings"
	"time"
)

// Request is what CURL sends.
type Request struct {
	Method  string
	URL     string
	Header  http.Header
	Body    string
	Timeout time.Duration // covers the whole request, including reading the body; 0 means no limit
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

	resp, err := (&http.Client{Timeout: r.Timeout}).Do(req)
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
