package main

import (
	"bytes"
	"fmt"
	"io"

	"os"
	"testing"
)

type File struct {
	fd int
}

func OpenFile(name string) (f *File, err error)       { return nil, nil }
func CloseFile(f *File) error                         { return nil }
func ReadFile(f *File, offset int64, data []byte) int { return 0 }
func (f *File) Close() error                          { return nil }
func (f *File) Read(offset int64, data []byte) int    { return 0 }

func main() {
	var closeF = (*File).Close
	var readF = (*File).Read
	f, _ := OpenFile("")
	readF(f, 0, nil)
	closeF(f)
	fmt.Fprintf(&UpperWriter{os.Stdout}, "hello world")

	var tb testing.TB = new(TB)
	tb.Fatal("Hello ,playground")
}

//--------------------
type UpperWriter struct {
	io.Writer
}

func (p *UpperWriter) Write(data []byte) (n int, err error) {
	return p.Writer.Write(bytes.ToUpper(data))
}

//----------------------
type TB struct {
	testing.TB
}

func (p *TB) Fatal(args ...interface{}) {
	fmt.Println("TB.Fatal disabled")
}

//----------------------
