package main

import (
	"fmt"
)

func main() {
	type Ta []int
	type Tb []int
	dest := Ta{1, 2, 3}
	src := Tb{5, 6, 7, 8, 9}
	n := copy(dest, src)
	fmt.Println(n, dest) //3 [5 6 7]
	n = copy(dest[1:], dest)
	fmt.Println(n, dest) //2 [5 5 6]

	a := [4]int{}
	n = copy(a[:], src)
	fmt.Println(n, a) //4 [5 6 7 8]
	n = copy(a[:], a[2:])
	fmt.Println(n, a) //2 [7 8 7 8]

	c := [3]int{1, 2, 3}
	for i := 0; i < len(c); i++ {
		c[i] = 1
	}
	fmt.Println(c)
	for _, it := range c {
		it = it + 9 //can not change value
	}
	fmt.Println(c)
}
