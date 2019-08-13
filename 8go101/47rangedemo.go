package main

import (
	"fmt"
)

var i int

func fa(s []int, n int) int {
	i = n
	for i = 0; i < len(s); i++ {
		fmt.Println("for:",i)
	}
	return i
}

func fb(s []int, n int) int {
	i = n
	for i = range s {
		fmt.Println("range:",i)
	}
	return i
}

func main() {
	s := []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
	fmt.Println(fa(s, -1), fb(s,-1))
	s = nil
	fmt.Println(fa(s,-1), fb(s, -1))

}
