package main

import (
	"fmt"
)

func main() {
	const N = 5
	for i := range [N]struct{}{} {
		fmt.Println(i)
	}

	for i := range [N][0]int{} {
		fmt.Println(i)
	}
	for i := range (*[N]int)(nil) {
		fmt.Println(i)
	}
}
