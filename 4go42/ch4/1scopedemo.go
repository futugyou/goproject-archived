package main

import (
	"fmt"
	"strings"
)

func main() {
	if a := 1; false {
	} else if b := 2; false {
	} else if c := 3; false {
	} else {
		println(a, b, c)
	}

	var b1 strings.Builder
	b1.WriteString("abc")
	b1.WriteString("def")
	println(b1.String())

	var arr1 = new([5]int)
	arr := arr1
	arr[2] = 100
	println(arr1[2], arr[2])
	var arr2 [5]int
	arrnew := arr2
	arr2[2] = 100
	println(arr2[2], arrnew[2])

	a := [5]int{1, 2, 3, 4, 5}
	t := a[1:3:5]
	fmt.Println(t)
}
