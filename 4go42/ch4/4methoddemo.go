package main

import (
	"fmt"
)

var G int = 7

func N() func(int) int {
	var i int
	return func(d int) int {
		fmt.Printf("i: %d, i address: %p\n", i, &i)
		i += d
		return i
	}
}
func main() {
	y := func() int {
		fmt.Printf("G: %d G address : %p\n", G, &G)
		G += 1
		return G
	}
	fmt.Println(y(), y)
	fmt.Println(y(), y)
	fmt.Println(y(), y)

	z := func() int {
		G += 1
		return G
	}()
	fmt.Println(z, &z)
	fmt.Println(z, &z)
	fmt.Println(z, &z)
	var f = N()
	fmt.Println(f(1), &f)
	fmt.Println(f(1), &f)
	fmt.Println(f(1), &f)
	var f1 = N()
	fmt.Println(f1(1), &f1)
}
