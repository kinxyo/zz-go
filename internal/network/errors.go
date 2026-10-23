package network

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// The kinds of failure CURL reports. Check for them with errors.Is; the cli turns each into its
// own exit code.
var (
	ErrRefused        = errors.New("connection refused")
	ErrNoSuchHost     = errors.New("no such host")
	ErrPlainHTTP      = errors.New("the server speaks plain HTTP")
	ErrCertificate    = errors.New("the TLS certificate was not accepted")
	ErrTimeout        = errors.New("timed out")
	ErrInvalidRequest = errors.New("the request could not be built")
)

// friendlyError shows a short message but keeps the original error underneath,
// so errors.Is and errors.As still see the cause. kind is one of the Err values above.
type friendlyError struct {
	msg  string
	kind error
	err  error
}

func (e *friendlyError) Error() string        { return e.msg }
func (e *friendlyError) Unwrap() error        { return e.err }
func (e *friendlyError) Is(target error) bool { return target == e.kind }

// explain turns the raw errors from net/http into short messages. host is the
// "host:port" that was requested, timeout the limit it ran with. Errors it doesn't
// recognise are returned as they are.
func explain(err error, host string, timeout time.Duration) error {
	friendly := func(kind error, format string, a ...any) error {
		return &friendlyError{msg: fmt.Sprintf(format, a...), kind: kind, err: err}
	}

	var dnsErr *net.DNSError
	var certErr *tls.CertificateVerificationError
	var netErr net.Error

	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return friendly(ErrRefused, "could not connect to %s (connection refused)", host)
	case errors.As(err, &dnsErr):
		return friendly(ErrNoSuchHost, "no such host: %s", dnsErr.Name)
	case errors.Is(err, http.ErrSchemeMismatch):
		return friendly(ErrPlainHTTP, "%s answered with plain HTTP; use http:// instead of https://", host)
	case errors.As(err, &certErr):
		return friendly(ErrCertificate, "TLS certificate for %s was not accepted: %v", host, certErr.Err)
	case errors.As(err, &netErr) && netErr.Timeout():
		if timeout > 0 {
			return friendly(ErrTimeout, "request to %s timed out after %s", host, timeout)
		}
		return friendly(ErrTimeout, "request to %s timed out", host)
	}

	return err
}
