package network

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"testing"
)

func TestExplain(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{
			// shaped like what net/http really returns
			"connection refused",
			&url.Error{Op: "Get", URL: "http://h:1/x", Err: &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}},
			"could not connect to h:1 (connection refused)",
		},
		{
			"unknown host",
			&url.Error{Op: "Get", URL: "http://h:1/x", Err: &net.DNSError{Name: "h.invalid", IsNotFound: true}},
			"no such host: h.invalid",
		},
		{
			"https to a plain http server",
			&url.Error{Op: "Get", URL: "https://h:1/x", Err: http.ErrSchemeMismatch},
			"h:1 answered with plain HTTP; use http:// instead of https://",
		},
		{
			"bad certificate",
			&url.Error{Op: "Get", URL: "https://h:1/x", Err: &tls.CertificateVerificationError{Err: errors.New("unknown authority")}},
			"TLS certificate for h:1 was not accepted: unknown authority",
		},
		{
			"timeout",
			&url.Error{Op: "Get", URL: "http://h:1/x", Err: os.ErrDeadlineExceeded},
			"request to h:1 timed out after 30s",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := explain(c.err, "h:1")
			if got.Error() != c.want {
				t.Errorf("explain() = %q; want %q", got.Error(), c.want)
			}
			// the friendly message must not hide the real cause
			if !errors.Is(got, c.err) {
				t.Errorf("original error is no longer reachable from %v", got)
			}
		})
	}
}

func TestExplainKeepsCause(t *testing.T) {
	err := &url.Error{Op: "Get", URL: "http://h:1/x", Err: &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}}
	if got := explain(err, "h:1"); !errors.Is(got, syscall.ECONNREFUSED) {
		t.Errorf("errors.Is(ECONNREFUSED) = false for %v", got)
	}
}

func TestExplainUnknownErrorPassesThrough(t *testing.T) {
	orig := errors.New("something else")
	if got := explain(orig, "h:1"); got != orig {
		t.Errorf("explain() = %v; want the same error back", got)
	}
}
