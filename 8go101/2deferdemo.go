package main

import (
	"fmt"
	"time"
)

func main() {
	func() {
		for i := 0; i < 3; i++ {
			defer fmt.Println("a:", i)
		}
	}()
	fmt.Println()
	func() {
		for i := 0; i < 3; i++ {
			defer func() {
				fmt.Println("b:", i)
			}()
		}
	}()
	fmt.Println()
	var a = 123
	go func(x int) {
		fmt.Println(x, a)
	}(a)
	a = 789
	time.Sleep(time.Second)
}
