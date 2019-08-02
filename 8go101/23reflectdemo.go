package main

import (
	"fmt"
	"reflect"
)

func main() {
	ta := reflect.ArrayOf(5, reflect.TypeOf(1))
	fmt.Println(ta)
	tc := reflect.ChanOf(reflect.SendDir, ta)
	fmt.Println(tc)
	tp := reflect.PtrTo(ta)
	fmt.Println(tp)
	ts := reflect.SliceOf(tp)
	fmt.Println(ts)
	tm := reflect.MapOf(ta, tc)
	fmt.Println(tm)
	tf := reflect.FuncOf([]reflect.Type{ta}, []reflect.Type{tp, tc}, false)
	fmt.Println(tf)
	tt := reflect.StructOf(
		[]reflect.StructField{
			{Name: "Age", Type: reflect.TypeOf(1)},
			//{Name: "name", Type: reflect.TypeOf("a"),PkgPath:"a"},  panic
		},
	)
	fmt.Println(tt)
}
