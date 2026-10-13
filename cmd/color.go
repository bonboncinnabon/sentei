package cmd

import "os"

// ANSI styles for CLI output. They are cleared at startup when stdout should
// not receive colour, so every print site stays plain without checking.
var (
	green  = "\033[0;32m"
	yellow = "\033[1;33m"
	blue   = "\033[0;34m"
	dim    = "\033[2m"
	nc     = "\033[0m"
)

func init() {
	if !colorEnabled(os.Stdout) {
		green, yellow, blue, dim, nc = "", "", "", "", ""
	}
}

// colorEnabled reports whether out should receive ANSI colour: it must be a
// terminal, and NO_COLOR (https://no-color.org) must be unset or empty.
func colorEnabled(out *os.File) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	// ponytail: any character device counts as a terminal, which includes
	// /dev/null. Use an isatty call if that distinction ever matters.
	info, err := out.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
