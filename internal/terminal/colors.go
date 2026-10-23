package terminal

import (
	"fmt"
	"io"
	"os"
)

type TextFmt string

const (
	Reset       TextFmt = "\x1b[0m"
	TextRed     TextFmt = "\x1b[31m"
	TextGreen   TextFmt = "\x1b[32m"
	TextYellow  TextFmt = "\x1b[33m"
	TextBlue    TextFmt = "\x1b[34m"
	TextMagenta TextFmt = "\x1b[35m"
	TextCyan    TextFmt = "\x1b[36m"
	TextBold    TextFmt = "\x1b[1m"
	TextDim     TextFmt = "\x1b[2m"
)

// useColor is true only when w is a terminal and NO_COLOR isn't set, so piped
// or redirected output stays free of escape codes.
func useColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	f, ok := w.(*os.File)
	return ok && isTerminal(f)
}

// isTerminal reports whether f is a terminal rather than a pipe or a file.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// StdoutIsTerminal reports whether stdout is a terminal.
func StdoutIsTerminal() bool {
	return isTerminal(os.Stdout)
}

// StdinIsTerminal reports whether stdin is a terminal, so nothing is being piped in.
func StdinIsTerminal() bool {
	return isTerminal(os.Stdin)
}

func colorFmt(std io.Writer, color TextFmt, format string, a ...any) {
	if !useColor(std) {
		fmt.Fprintf(std, format, a...)
		return
	}

	fmt.Fprintf(std, string(color)+format+string(Reset), a...)
}
