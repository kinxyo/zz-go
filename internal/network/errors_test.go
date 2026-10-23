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
		kind error
	}{
		{
			// shaped like what net/http really returns
			"connection refused",
			&url.Error{Op: "Get", URL: "http://h:1/x", Err: &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}},
			"could not connect to h:1 (connection refused)",
			ErrRefused,
		},
		{
			"unknown host",
			&url.Error{Op: "Get", URL: "http://h:1/x", Err: &net.DNSError{Name: "h.invalid", IsNotFound: true}},
			"no such host: h.invalid",
			ErrNoSuchHost,
		},
		{
			"https to a plain http server",
			&url.Error{Op: "Get", URL: "https://h:1/x", Err: http.ErrSchemeMismatch},
			"h:1 answered with plain HTTP; use http:// instead of https://",
			ErrPlainHTTP,
		},
		{
			"bad certificate",
			&url.Error{Op: "Get", URL: "https://h:1/x", Err: &tls.CertificateVerificationError{Err: errors.New("unknown authority")}},
			"TLS certificate for h:1 was not accepted: unknown authority",
			ErrCertificate,
		},
		{
			"timeout",
			&url.Error{Op: "Get", URL: "http://h:1/x", Err: os.ErrDeadlineExceeded},
			"request to h:1 timed out after 30s",
			ErrTimeout,
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
			// it is its own kind and no other, so the cli picks one exit code
			for _, kind := range allKinds {
				if want := kind == c.kind; errors.Is(got, kind) != want {
					t.Errorf("errors.Is(got, %q) = %v; want %v", kind, !want, want)
				}
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
	got := explain(orig, "h:1")
	if got != orig {
		t.Errorf("explain() = %v; want the same error back", got)
	}
	for _, kind := range allKinds {
		if errors.Is(got, kind) {
			t.Errorf("an unknown error should have no kind, but is %q", kind)
		}
	}
}

var allKinds = []error{ErrRefused, ErrNoSuchHost, ErrPlainHTTP, ErrCertificate, ErrTimeout, ErrInvalidRequest}
