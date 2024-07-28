package main

import (
	"fmt"
)

func main() {
	defer fmt.Println(9)
	fmt.Println(0)
	defer fmt.Println(8)
	fmt.Println(1)
	if false {
		defer fmt.Println("not reachable")
	}
	defer func() {
		defer fmt.Println(7)
		fmt.Println(3)
		defer func() {
			fmt.Println(5)
			fmt.Println(6)
		}()
		fmt.Println(4)
	}()
	fmt.Println(2)
	return
	defer fmt.Println("not reach")
}
