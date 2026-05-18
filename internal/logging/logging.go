package logging

import (
	"fmt"
	"os"
)

// ANSI color codes for terminal output.
const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorGray   = "\033[90m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
)

// Logger provides leveled, colored terminal output.
type Logger struct {
	verbose bool
}

// New creates a Logger. When verbose is true, Detail messages are shown.
func New(verbose bool) *Logger {
	return &Logger{verbose: verbose}
}

// Step prints a major phase marker. Equivalent to "Starting SPIRE server..."
func (l *Logger) Step(msg string) {
	fmt.Fprintf(os.Stderr, "\n%s>>>  %s%s\n", colorCyan, msg, colorReset)
}

// Info prints general information.
func (l *Logger) Info(msg string) {
	fmt.Fprintf(os.Stderr, "%s   %s%s\n", colorWhite, msg, colorReset)
}

// Infof prints formatted general information.
func (l *Logger) Infof(format string, args ...any) {
	l.Info(fmt.Sprintf(format, args...))
}

// Detail prints debug output, only visible in verbose mode.
func (l *Logger) Detail(msg string) {
	if l.verbose {
		fmt.Fprintf(os.Stderr, "%s   [debug] %s%s\n", colorGray, msg, colorReset)
	}
}

// Detailf prints formatted debug output.
func (l *Logger) Detailf(format string, args ...any) {
	l.Detail(fmt.Sprintf(format, args...))
}

// Ok prints a success confirmation with a checkmark.
func (l *Logger) Ok(msg string) {
	fmt.Fprintf(os.Stderr, "%s   OK %s%s\n", colorGreen, msg, colorReset)
}

// Warn prints a non-fatal warning.
func (l *Logger) Warn(msg string) {
	fmt.Fprintf(os.Stderr, "%s   WARN %s%s\n", colorYellow, msg, colorReset)
}

// Error prints a significant error message.
func (l *Logger) Error(msg string) {
	fmt.Fprintf(os.Stderr, "%s   ERROR %s%s\n", colorRed, msg, colorReset)
}

// Errorf prints a formatted error message.
func (l *Logger) Errorf(format string, args ...any) {
	l.Error(fmt.Sprintf(format, args...))
}
