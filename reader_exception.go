package gozxing

import (
	"fmt"

	"github.com/springsunx/ean13-api/xerr"
)

type ReaderException interface {
	error
	readerException()
}

type readerException struct {
	exception
}

func WrapReaderException(e error) ReaderException {
	return readerException{
		wrapException("ReaderException", e),
	}
}

func (readerException) readerException() {}

type exception struct {
	msg   string
	next  error
	frame xerr.Frame
}

func newException(prefix string, args ...interface{}) exception {
	msg := prefix
	if len(args) > 0 {
		msg += ": " + fmt.Sprintf(args[0].(string), args[1:]...)
	}
	return exception{
		msg,
		nil,
		xerr.Caller(3),
	}
}

func wrapException(msg string, next error) exception {
	return exception{
		msg,
		next,
		xerr.Caller(3),
	}
}

func (e exception) Error() string {
	return e.msg
}

func (e exception) Unwrap() error {
	return e.next
}

func (e exception) Format(s fmt.State, v rune) {
	xerr.FormatError(e, s, v)
}

func (e exception) FormatError(p xerr.Printer) error {
	p.Print(e.msg)
	xerr.FormatFrame(e.frame, p)
	return e.next
}
