package main

import (
	"fmt"
)

func main() {
	a := make([]int, 9)
	fmt.Printf("%#v\t cap(a):", a)
	fmt.Println(cap(a))
	b := a[2:]
	fmt.Printf("%#v\t cap(b):", b)
	fmt.Println(cap(b))

	//--append
	c := []int{1, 2, 3, 4, 5}
	c = append(c, 0)
	copy(c[3+1:], c[3:])
	c[3] = 99
	fmt.Printf("%#v\t cap(c):%d\n", c, cap(c))

	//--delete head
	c = append(c[:0], c[1:]...)
	fmt.Printf("%#v\t cap(c):%d\n", c, cap(c))
	c = c[:copy(c, c[1:])]
	fmt.Printf("%#v\t cap(c):%d\n", c, cap(c))

	//--delete mid
	c = c[:2+copy(c[2:], c[2+1:])]
	fmt.Printf("%#v\t cap(c):%d\n", c, cap(c))


}
