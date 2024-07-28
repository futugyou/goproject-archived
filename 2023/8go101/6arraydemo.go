package main

import (
	"fmt"
)

func main() {
	a := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	from := 2
	to := 4
	f := a[from:]
	fmt.Println("from:", f)
	t := a[to:]
	fmt.Println("to:", t)
	n := copy(f, t)
	fmt.Println(n, f, a)
	a = a[:from+n]
	fmt.Println(a)
	fmt.Println()
	aClone:= append(a[:0:0],a...)
	fmt.Println(a,aClone)
}
