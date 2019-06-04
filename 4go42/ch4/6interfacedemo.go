package main

import (
	"fmt"
)

type I interface {
	f()
}

type T string

func (t T) f() {
	fmt.Println("T method")
}

type Stringer interface {
	String() string
}

func main() {
	var varI I
	varI = T("TString")
	if v, ok := varI.(T); ok {
		fmt.Println("varI type is :", v)
		varI.f()
	}

	var value interface{}

	switch str := value.(type) {
	case string:
		fmt.Println("value is string", str)
	case Stringer:
		fmt.Println("value is Stringer", str)
	default:
		fmt.Println("value type is not in this")
	}

	value = "12345"
	str,ok:=value.(string)
	if ok{
		fmt.Printf("value type is %T\n",str)
	}else{
		fmt.Println("value is no string")
	}
}
