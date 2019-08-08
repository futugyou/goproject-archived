package main

import (
	"fmt"
	"sync/atomic"
)

func main() {
	type T struct{ a, b, c int }
	var ta = T{1, 2, 3}
	var v atomic.Value
	v.Store(ta)
	var tb = v.Load().(T)
	fmt.Println(tb)
	fmt.Println(ta == tb)
}
