package cli

import (
	"errors"

	"github.com/kinxyo/zz-go/internal/network"
)

// Exit codes. Where curl has a code for the same situation, zz uses the same number, so a script
// written for curl reads zz the same way. ExitCodes below is the reference list: the README and
// the help screen show it, and tests check that both list every code.
const (
	exitOK            = 0
	exitFailure       = 1
	exitUsage         = 2
	exitNoSuchHost    = 6
	exitRefused       = 7
	exitHTTPError     = 22
	exitOutputRefused = 23
	exitTimeout       = 28
	exitPlainHTTP     = 35
	exitCertificate   = 60
)

// ExitCode is one row of the exit code table.
type ExitCode struct {
	Code    int
	Meaning string
}

// ExitCodes is every exit code zz uses, in order.
var ExitCodes = []ExitCode{
	{exitOK, "a response arrived (any status, unless -f)"},
	{exitFailure, "any other failure"},
	{exitUsage, "bad arguments: missing URL, bad -H, bad body, unknown method or flag, malformed URL"},
	{exitNoSuchHost, "no such host"},
	{exitRefused, "connection refused"},
	{exitHTTPError, "-f and the status was 4xx or 5xx"},
	{exitOutputRefused, "a binary body was not written to a terminal"},
	{exitTimeout, "the request timed out"},
	{exitPlainHTTP, "https:// to a server that speaks plain HTTP"},
	{exitCertificate, "the server's TLS certificate was not accepted"},
}

// exitCode picks the exit code for a request that failed.
func exitCode(err error) int {
	switch {
	case errors.Is(err, network.ErrInvalidRequest):
		return exitUsage
	case errors.Is(err, network.ErrNoSuchHost):
		return exitNoSuchHost
	case errors.Is(err, network.ErrRefused):
		return exitRefused
	case errors.Is(err, network.ErrTimeout):
		return exitTimeout
	case errors.Is(err, network.ErrPlainHTTP):
		return exitPlainHTTP
	case errors.Is(err, network.ErrCertificate):
		return exitCertificate
	default:
		return exitFailure
	}
}
