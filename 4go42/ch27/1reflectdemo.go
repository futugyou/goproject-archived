package main

import (
	"fmt"
	"reflect"
)

type Student struct {
	name string
	Age  int `json:"years"`
}

func main() {
	var a int = 50
	v := reflect.ValueOf(a)
	t := reflect.TypeOf(a)
	fmt.Println(v, t, v.Type(), t.Kind(), reflect.ValueOf(&a).Elem())
	seta := reflect.ValueOf(&a).Elem()
	fmt.Println(seta, seta.CanSet())
	seta.SetInt(10000)
	fmt.Println(seta)

	var b [5]int = [5]int{5, 6, 7, 8}
	fmt.Println(reflect.TypeOf(b),
		reflect.TypeOf(b).Kind(),
		reflect.TypeOf(b).Elem())

	var stu Student = Student{"tom", 18}
	p := reflect.ValueOf(stu)
	fmt.Println(p.Type())
	fmt.Println(p.Kind())

	setStudent := reflect.ValueOf(&stu).Elem()
	var nameField = setStudent.Field(0)
	fmt.Println("name field can set :", nameField.CanSet())
	setStudent.Field(1).SetInt(99)
	fmt.Println(setStudent)

	tagAge, _ := setStudent.Type().FieldByName("Age")
	fmt.Println(tagAge.Tag.Get("json"))
}
