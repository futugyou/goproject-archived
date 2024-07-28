package main

import (
	"fmt"
)

type A int
type B int

func (b B) M(x int) string {
	return ""
}

func check(v interface{}) bool {
	_, has := v.(interface{ M(int) string })
	return has
}
func main() {
	var a A = 123
	var b B = 234
	fmt.Println(check(a))
	fmt.Println(check(b))

	fmt.Println(^uint(0))
	fmt.Println(^int(0))
	fmt.Println(int(^uint(0) >> 1))

	fmt.Println(^uint(0) >> 63)
	fmt.Println(32 << (^uint(0) >> 63))
}
