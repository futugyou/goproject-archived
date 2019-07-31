package main

import (
	"fmt"
	"unsafe"
)

func main() {
	var x struct {
		a string
		b bool
		c int64
	}
	const M, N = unsafe.Sizeof(x.a), unsafe.Sizeof(x)
	fmt.Println(M, N)
	fmt.Println(unsafe.Alignof(x.a))
	fmt.Println(unsafe.Alignof(x.b))
	fmt.Println(unsafe.Alignof(x.c))
	fmt.Println(unsafe.Offsetof(x.a))
	fmt.Println(unsafe.Offsetof(x.b))
	fmt.Println(unsafe.Offsetof(x.c))
}
