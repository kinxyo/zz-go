package network

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
)

// friendlyError shows a short message but keeps the original error underneath,
// so errors.Is and errors.As still see the cause.
type friendlyError struct {
	msg string
	err error
}

func (e *friendlyError) Error() string { return e.msg }
func (e *friendlyError) Unwrap() error { return e.err }

// explain turns the raw errors from net/http into short messages. host is the
// "host:port" that was requested. Errors it doesn't recognise are returned as they are.
func explain(err error, host string) error {
	friendly := func(format string, a ...any) error {
		return &friendlyError{msg: fmt.Sprintf(format, a...), err: err}
	}

	var dnsErr *net.DNSError
	var certErr *tls.CertificateVerificationError
	var netErr net.Error

	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return friendly("could not connect to %s (connection refused)", host)
	case errors.As(err, &dnsErr):
		return friendly("no such host: %s", dnsErr.Name)
	case errors.Is(err, http.ErrSchemeMismatch):
		return friendly("%s answered with plain HTTP; use http:// instead of https://", host)
	case errors.As(err, &certErr):
		return friendly("TLS certificate for %s was not accepted: %v", host, certErr.Err)
	case errors.As(err, &netErr) && netErr.Timeout():
		return friendly("request to %s timed out after %s", host, requestTimeout)
	}

	return err
}
