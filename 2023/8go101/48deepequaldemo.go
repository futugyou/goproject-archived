package main

import (
	"fmt"
	"reflect"
)

func main() {
	type Book struct{ page int }
	x := struct{ page int }{123}
	y := Book{123}

	fmt.Println(reflect.DeepEqual(x, y))
	fmt.Println(x == y)

	z := Book{123}
	fmt.Println(reflect.DeepEqual(&z, &y))
	fmt.Println(&z == &y)

	type T struct{ p *T }
	t := &T{&T{nil}}
	t.p.p = t
	fmt.Println(reflect.DeepEqual(t, t.p))
	fmt.Println(t == t.p)

	var f1, f2 func() = nil, func() {}
	fmt.Println(reflect.DeepEqual(f1, f1))
	fmt.Println(reflect.DeepEqual(f2, f2))

	var a, b interface{} = []int{1, 2}, []int{1, 2}
	fmt.Println(reflect.DeepEqual(a, b))
	//fmt.Println(a==b)
}
