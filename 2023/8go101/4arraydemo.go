package main

import (
	"fmt"
)

func main() {
	type Person struct {
		name string
		age  int
	}

	persons := [2]Person{{"name1", 29}, {"name2", 30}}
	for i, p := range persons {
		fmt.Println(i, p)
		//0 {name1 29}
		//1 {name2 30}
		persons[1].name = "aaaaa"
		p.age = 99
	}
	fmt.Println("persons:", &persons)//persons: &[{name1 29} {aaaaa 30}]
	persons2 := []Person{{"name1", 29}, {"name2", 30}}
	for i, p := range persons2 {
		fmt.Println(i, p)
		//0 {name1 29}
		//1 {aaaaa 30}
		persons2[1].name = "aaaaa"
		p.age = 99
	}
	fmt.Println("persons2:", &persons2)//persons: &[{name1 29} {aaaaa 30}]
}
