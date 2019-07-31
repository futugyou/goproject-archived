package main

import (
	"fmt"
	"reflect"
)

type F func(int) bool

func (f F) Validate(n int) bool {
	return f(n)
}
func (f *F) Modify(f2 F) {
	*f = f2
}

type B bool

func (b B) IsTrue() bool { return bool(b) }
func (p *B) Invert() {
	*p = !*p
}

type I interface {
	Load()
	Save()
}

func PrinttypeMethods(t reflect.Type) {
	fmt.Println(t, "has", t.NumMethod(), "methods:")
	for i := 0; i < t.NumMethod(); i++ {
		fmt.Print(" method#", i, ": ", t.Method(i).Name, "\n")
	}
}

func main() {
	var s struct {
		F
		*B
		I
	}

	PrinttypeMethods(reflect.TypeOf(s))
	fmt.Println()
	PrinttypeMethods(reflect.TypeOf(&s))

}
