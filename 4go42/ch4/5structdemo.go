package main

import (
	"fmt"
	"reflect"
)

type Writer interface {
	Write()
}

type Author struct {
	name string
	Writer
}

type Other struct {
	i int
}

func (a Author) Write() {
	fmt.Println(a.name, " write.")
}

func (o Other) Write() {
	fmt.Println(" other write.")
}

type Student struct{
	name string "student name"
	Age int "student age"
	Room int `json:"Room"`
}

func main() {
	Ao := Author{"other", Other{99}}
	Ao.Write()

	Ao = Author{name: "123"}
	Ao.Write()

	st:= Student{"operation",13,139}
	fmt.Println(reflect.TypeOf(st).Field(0).Tag)
	fmt.Println(reflect.TypeOf(st).Field(1).Tag)
	fmt.Println(reflect.TypeOf(st).Field(2).Tag)
}
