package main

import (
	"fmt"
	"unsafe"
	//"math"
)

func main() {
	fmt.Println(unsafe.Sizeof(float64(0)))
	fmt.Println(unsafe.Sizeof(float32(0)))
	fmt.Println(unsafe.Sizeof(string("sssss")))
	fmt.Println(unsafe.Sizeof(int16(0)))
	fmt.Println(unsafe.Sizeof(int(0)))

	fmt.Println()
	fmt.Println(unsafe.Sizeof(x), unsafe.Alignof(x))
	fmt.Println(unsafe.Sizeof(x.a), unsafe.Alignof(x.a), unsafe.Offsetof(x.a))
	fmt.Println(unsafe.Sizeof(x.b), unsafe.Alignof(x.b), unsafe.Offsetof(x.b))
	fmt.Println(unsafe.Sizeof(x.c), unsafe.Alignof(x.c), unsafe.Offsetof(x.c))
	fmt.Println()

	fmt.Printf("%#016x\n", float64bits(1.0))
	pb := (*int16)(unsafe.Pointer(uintptr(unsafe.Pointer(&x)) + unsafe.Offsetof(x.b)))
	*pb = 42
	fmt.Println(x.b)
}

var x struct {
	a bool
	b int16
	c []int
}

func float64bits(f float64) uint64 {
	return *(*uint64)(unsafe.Pointer(&f))
}
