package gozxing

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func newTestException(args ...interface{}) ReaderException {
	return readerException{
		newException("TestException", args...),
	}
}

func TestException_Format(t *testing.T) {
	re := newTestException("%d %x", 10, 10)

	s := fmt.Sprintf("%+v", re)
	cases := []string{
		"TestException: 10 a:",
		"ean13-api.TestException_Format",
		"reader_exception_test.go:17",
	}
	for _, c := range cases {
		if strings.Index(s, c) < 0 {
			t.Fatalf("error message must contains \"%s\"\n%s", c, s)
		}
	}
}

func TestWrapReaderException(t *testing.T) {
	base := errors.New("base error")
	e := WrapReaderException(base)

	if !errors.Is(e, base) {
		t.Fatalf("err is not base")
	}

	s := fmt.Sprintf("%+v", e)
	cases := []string{
		"base error",
		"ean13-api.TestWrapReaderException",
		"reader_exception_test.go:34",
	}
	for _, c := range cases {
		if strings.Index(s, c) < 0 {
			t.Fatalf("error message must contains \"%s\"\n%s", c, s)
		}
	}

	e.readerException()
}
