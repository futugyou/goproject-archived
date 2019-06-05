package main

import (
	"fmt"
	"reflect"
)

type ss struct {
	int
	string
	bool
	float64
}

func (s ss) Method1(i int) string {
	return "method1"
}
func (s *ss) Method2(i int) string {
	return "method2"
}

var (
	structValue = ss{
		20,
		"struct",
		false,
		64.0,
	}
)

var complexTypes = []interface{}{
	structValue, &structValue,
	structValue.Method1, structValue.Method2,
}

func main() {
	for i := 0; i < len(complexTypes); i++ {
		PrintInfo(complexTypes[i])
	}
}

func PrintInfo(i interface{}) {
	if i == nil {
		fmt.Println("---------------------")
		fmt.Printf("invaid value: %v\n", i)
		fmt.Println("---------------------")
	}

	v := reflect.ValueOf(i)
	PrintValue(v)
}

func PrintValue(v reflect.Value) {
	fmt.Println("---------------------")
	fmt.Println("String  :", v.String())
	fmt.Println("Type    :", v.Type())
	fmt.Println("Kind    :", v.Kind())
	fmt.Println("CanAddr :", v.CanAddr())
	fmt.Println("CanSet  :", v.CanSet())
	if v.CanAddr() {
		fmt.Println("addr    :", v.Addr())
		fmt.Println("UnsafeAddr:", v.UnsafeAddr())
	}
	fmt.Println("NumMethod :", v.NumMethod())
	if v.NumMethod() > 0 {
		i := 0
		for ; i < v.NumMethod()-1; i++ {
			fmt.Println("    ┣ %v\n", v.Method(i).String())
		}
		fmt.Printf("    ┗ %v\n", v.Method(i).String())
		fmt.Println("MethodByName :", v.MethodByName("Method2").String())
	}

	switch v.Kind() {
	case reflect.Struct:
		fmt.Println("-------struct-------")
		fmt.Println("NumField  :", v.NumField())
		if v.NumField() > 0 {
			var i int
			for i = 0; i < v.NumField()-1; i++ {
				field := v.Field(i)
				fmt.Printf("    ├ %-8v %v\n", field.Type(), field.String())
			}
			field := v.Field(i)
			fmt.Printf("    └ %-8v %v\n", field.Type(), field.String())
			if v := v.FieldByName("ptr"); v.IsValid() {
				fmt.Println("FieldByName(ptr)   :", v.Type().Name())
			}
			v := v.FieldByNameFunc(func(s string) bool { return len(s) > 3 })
			if v.IsValid() {
				fmt.Println("FieldByNameFunc    :", v.Type().Name())
			}
		}
	}
}
