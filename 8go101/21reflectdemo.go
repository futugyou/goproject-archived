package main

import (
	"fmt"
	"reflect"
)

type T []interface {
	m()
}

func (T) m() {

}

func main() {
	tp := reflect.TypeOf(new(interface{}))
	tt := reflect.TypeOf(T{})
	fmt.Println(tp.Kind(), tt.Kind())

	ti, tim := tp.Elem(), tt.Elem()
	fmt.Println(ti.Kind(), tim.Kind())

	fmt.Println(tt.Implements(tim))
	fmt.Println(tp.Implements(tim))
	fmt.Println(tim.Implements(tim))

	
	fmt.Println(tt.Implements(ti))
	fmt.Println(tp.Implements(ti))
	fmt.Println(tim.Implements(ti))
	fmt.Println(ti.Implements(ti))
}
