package terminal

import (
	"fmt"
	"os"
)

func Err(format string, a ...any) {
	colorFmt(os.Stderr, TextRed, format, a...)
}

func Fatal(format string, a ...any) {
	Err(format, a...)
	Exit()
}

// FatalWith prints the error and ends the program with the given exit status.
func FatalWith(code int, format string, a ...any) {
	Err(format, a...)
	ExitWith(code)
}

// `Panic()` is not meant for production use;
// It's more of a placeholder for adding error handling later.
func Panic(a ...any) {
	fmt.Fprintln(os.Stderr, a...)
	Exit()
}
