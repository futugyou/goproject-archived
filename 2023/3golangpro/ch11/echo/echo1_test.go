package main

import (
	"bytes"
	"fmt"
	"testing"
)

func testEcho(t *testing.T) {
	var tests = []struct {
		newline bool
		sep     string
		args    []string
		want    string
	}{
		{true, "", []string{}, "\n"},
		{false, "", []string{}, ""},
		{true, "\t", []string{"one", "two", "three"}, "one\ttwo\tthress\n"},
		{true, "", []string{"a", "b", "c"}, "a,b,c\n"},
		{false, ":", []string{"1", "2", "3"}, "1:2:3"},
	}
	for _, test := range tests {
		desc := fmt.Sprintf("echo(%v ,%q, %q)",
			test.newline, test.sep, test.args)

		out = new(bytes.Buffer)
		if err := echo(test.newline, test.sep, test.args); err != nil {
			t.Errorf("%s failed: %v", desc, err)
			continue
		}
		got := out.(*bytes.Buffer).String()
		if got != test.want {
			t.Errorf("%s = %q, want %q", desc, got, test.want)
		}
	}
}
