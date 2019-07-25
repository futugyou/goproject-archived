package main

import (
	"bytes"
	"fmt"
	"unicode/utf8"
)

func Runes2Bytes(rs []rune) []byte {
	n := 0
	for _, r := range rs {
		n += utf8.RuneLen(r)
	}
	n, bs := 0, make([]byte, n)
	for _, r := range rs {
		n += utf8.EncodeRune(bs[n:], r)
	}
	return bs
}

func main() {
	s := "法规hi欧元体验和教科书里的"
	bs := []byte(s)
	fmt.Println("[]byte:", bs)
	s = string(bs)
	fmt.Println(s)
	rs := []rune(s)
	fmt.Println("[]rune:", rs)
	s = string(rs)
	fmt.Println(s)
	rs = bytes.Runes(bs)
	fmt.Println("[]rune:", rs)
	bs = Runes2Bytes(rs)
	fmt.Println("[]byte:", bs)
}
