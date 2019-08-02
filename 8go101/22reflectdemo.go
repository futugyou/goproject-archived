package main

import (
	"fmt"
	"reflect"
)

type F func(string, int) bool

func (f F) Validate(s string) bool {
	return f(s, 32)
}

func main() {
	var x struct {
		n int
		f F
	}

	tx := reflect.TypeOf(x)
	fmt.Println(tx.Kind())
	fmt.Println(tx.NumField())
	ft := tx.Field(1).Type
	fmt.Println(ft.Kind())
	fmt.Println(ft.IsVariadic())
	fmt.Println(ft.NumIn(), ft.NumOut())
	fmt.Println((ft.NumMethod()))
	ts, ti, tb := ft.In(0), ft.In(1), ft.Out(0)
	fmt.Println(ts.Kind(), ti.Kind(), tb.Kind())
}
