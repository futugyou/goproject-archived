package main

import (
	"fmt"
	"unsafe"
)

type T struct {
	x bool
	y [3]int16
}

const N = unsafe.Offsetof(T{}.y)
const M = unsafe.Sizeof([3]int16{}[0])

func main() {
	type mystring string
	ms := []mystring{"a", "b", "v"}
	fmt.Printf("%s\n", ms)
	fmt.Println(&ms)
	fmt.Println(unsafe.Pointer(&ms))
	ss := *(*[]string)(unsafe.Pointer(&ms))
	ss[1] = "vvv"
	fmt.Printf("%s\n", ms)
	ms = *(*[]mystring)(unsafe.Pointer(&ss))
	fmt.Printf("%s\n", ms)

	var f float64 = -987.99
	var ui, uu = *(*uint64)(unsafe.Pointer(&f)), uint64(f)
	f, ff := *(*float64)(unsafe.Pointer(&ui)), float64(uu)
	fmt.Println(ui, uu, f, ff)

	t := T{x: true, y: [3]int16{123, 456, 789}}
	p := unsafe.Pointer(&t)
	fmt.Println(N, M, p)
	tp2 := (*int16)(unsafe.Pointer(uintptr(p) + N + M + M))
	fmt.Println(*tp2)
}
