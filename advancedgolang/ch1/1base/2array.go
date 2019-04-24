package main

import "fmt"

func main() {
	var a [3]int
	fmt.Println(a)
	var b = [...]int{1, 2, 3}
	fmt.Println(b)
	var c = [...]int{2: 3, 4: 5}
	fmt.Println(c)
	fmt.Printf("c :%#v\n", c)
}
