package main

import (
	"fmt"
	"reflect"
	"unsafe"
)

func main() {
	s := "hello, world"
	hello := s[:5]
	world := s[7:]
	s1 := "hello, world"[:5]
	s2 := "hello, world"[7:]
	fmt.Println(hello, world)
	fmt.Println("len(s):", (*reflect.StringHeader)(unsafe.Pointer(&s)).Len)
	fmt.Println("len(s1):", (*reflect.StringHeader)(unsafe.Pointer(&s1)).Len)
	fmt.Println("len(s2):", (*reflect.StringHeader)(unsafe.Pointer(&s2)).Len)
	fmt.Printf("%#v\n", []byte("世界")) //[]byte{0xe4, 0xb8, 0x96, 0xe7, 0x95, 0x8c}

	fmt.Println()
	for i, c := range "\x00\xb8\x96\xe7\x95\x8c" {
		fmt.Println(i, c)
	}
	fmt.Println()
	for i, c := range []byte("世界abc") {
		fmt.Println(i, c)
	}
	fmt.Println()
	const ss = "\xe4\x00\x00\xe7\x95\x8cabc"
	for i := 0; i < len(ss); i++ {
		fmt.Printf("%d %x\n", i, ss[i])
	}
	fmt.Println()
	fmt.Printf("%#v\n", []rune("世界"))             // []int32{19990, 30028}
	fmt.Printf("%#v\n", string([]rune{'世', '界'})) // 世界
}
