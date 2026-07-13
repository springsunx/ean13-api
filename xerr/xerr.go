// Package xerr provides error formatting utilities compatible with
// the deprecated golang.org/x/xerrors API, using only stdlib.
package xerr

import (
	"fmt"
	"runtime"
	"strconv"
)

// Frame holds a single stack frame.
type Frame struct {
	pc       uintptr
	file     string
	line     int
	function string
}

// Caller returns a Frame recording the caller at skip levels up.
func Caller(skip int) Frame {
	var pcs [1]uintptr
	runtime.Callers(skip+1, pcs[:])
	frames := runtime.CallersFrames(pcs[:])
	frame, _ := frames.Next()
	return Frame{
		pc:       frame.PC,
		file:     frame.File,
		line:     frame.Line,
		function: frame.Function,
	}
}

// Format writes the frame to a fmt.State using the given format rune.
func (f Frame) Format(s fmt.State, v rune) {
	if f.file == "" {
		fmt.Fprintf(s, "unknown")
		return
	}
	switch v {
	case 's':
		if s.Flag('+') {
			fmt.Fprintf(s, "%s\n\t%s", f.function, f.file)
		} else {
			fmt.Fprint(s, f.file)
		}
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%s\n\t%s:%d", f.function, f.file, f.line)
		} else {
			fmt.Fprintf(s, "%s:%d", f.file, f.line)
		}
	case 'd':
		fmt.Fprint(s, f.line)
	}
}

// Printer is the interface used by FormatError.
type Printer interface {
	Print(args ...interface{})
	Printf(format string, args ...interface{})
	Detail() bool
}

type printer struct {
	state fmt.State
	buf   []byte
	detail bool
}

func (p *printer) Print(args ...interface{}) {
	fmt.Fprint(p, args...)
}

func (p *printer) Printf(format string, args ...interface{}) {
	fmt.Fprintf(p, format, args...)
}

func (p *printer) Detail() bool {
	return p.detail
}

func (p *printer) Write(b []byte) (int, error) {
	p.buf = append(p.buf, b...)
	return len(b), nil
}

// FormatError formats an error with optional stack trace detail.
func FormatError(err error, s fmt.State, v rune) {
	// Check if the error implements Formatter
	type formatter interface {
		FormatError(p Printer) error
	}

	p := &printer{state: s, detail: s.Flag('+')}

	if f, ok := err.(formatter); ok {
		next := f.FormatError(p)
		if next != nil {
			if p.detail {
				p.Print("\n")
			} else {
				p.Print(": ")
			}
			formatErrorTo(p, next, v)
		}
	} else {
		p.Print(err.Error())
	}

	s.Write(p.buf)
}

func formatErrorTo(p *printer, err error, v rune) {
	type formatter interface {
		FormatError(p Printer) error
	}

	if f, ok := err.(formatter); ok {
		next := f.FormatError(p)
		if next != nil {
			if p.detail {
				p.Print("\n")
			} else {
				p.Print(": ")
			}
			formatErrorTo(p, next, v)
		}
	} else {
		p.Print(err.Error())
	}
}

// FormatFrame formats a single frame for use in error output.
func FormatFrame(f Frame, p Printer) {
	if p.Detail() {
		if f.file != "" {
			p.Printf(":\n\t%s:%d", f.file, f.line)
			if f.function != "" {
				p.Printf(" (%s)", f.function)
			}
		}
	}
}

// FormatInt formats an integer to string (used in tests).
func FormatInt(n int) string {
	return strconv.Itoa(n)
}
