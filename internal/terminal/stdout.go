package terminal

import (
	"fmt"
	"os"
)

// TODO: maybe check error `fprintf`?

// Print -- Format
func PrintF(format string, a ...any) {
	_, _ = fmt.Fprintf(os.Stdout, format, a...)
}

// Print -- Format & Color
func PrintFC(color TextFmt, format string, a ...any) {
	colorFmt(os.Stdout, color, format, a...)
}

// Print -- NewLine
func Print(a ...any) {
	_, _ = fmt.Fprintln(os.Stdout, a...)
}
