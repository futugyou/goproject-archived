package main

import (
	"fmt"
	"reflect"
)

func main() {
	n := 123
	p := &n
	vp := reflect.ValueOf(p)
	fmt.Println(vp, vp.CanSet(), vp.CanAddr())
	vn := reflect.Indirect(vp) //vp.Elem()
	fmt.Println(vn, vn.CanSet(), vn.CanAddr(), vn.Addr())
	vn.Set(reflect.ValueOf(900))
	fmt.Println(n)
}
