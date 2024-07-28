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
	const M, N, NN, MM = unsafe.Sizeof(x.a), unsafe.Sizeof(x.b), unsafe.Sizeof(x.c), unsafe.Sizeof(x)
	fmt.Println(M, N, NN, MM)
	fmt.Println(unsafe.Alignof(x.a))
	fmt.Println(unsafe.Alignof(x.b))
	fmt.Println(unsafe.Alignof(x.c))
	fmt.Println(unsafe.Offsetof(x.a))
	fmt.Println(unsafe.Offsetof(x.b))
	fmt.Println(unsafe.Offsetof(x.c)) 

	xx := 123
	fmt.Println(&xx)
	p := unsafe.Pointer(&xx)
	fmt.Println(p)
	pp := &p
	fmt.Println(pp)
	p = unsafe.Pointer(pp)
	fmt.Println(p)
	pp = (*unsafe.Pointer)(p)
	fmt.Println(pp)
}
