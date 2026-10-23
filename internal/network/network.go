package network

import (
	"io"
	"net/http"
	"strings"
	"time"
)

type Result struct {
	Method     string
	URL        string
	Status     string // "200 OK"
	StatusCode int
	Header     http.Header
	Body       []byte
	Elapsed    time.Duration
}

func CURL(method string, url string, headers http.Header, payload string) (*Result, error) {

	start := time.Now() // -- STARTS
	// +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
	p := strings.NewReader(payload)
	req, err := http.NewRequest(method, url, p)
	if err != nil {
		return nil, err
	}

	for k, vv := range headers {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, explain(err, req.URL.Host)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
	elapsed := time.Since(start) // -- ENDS

	return &Result{
		Method:     method,
		URL:        url,
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       body,
		Elapsed:    elapsed,
	}, nil
}
