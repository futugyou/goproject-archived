package main

import "fmt"

func main() {
	i := 10
	zeroval(i)
	fmt.Println(i)
	zeropoint(&i)
	fmt.Println(i)
	fmt.Println(&i)

	fmt.Println(person{name: "2", age: 10})
	s := person{name: "22", age: 10}
	s.age = 19
	fmt.Println(s)
	sp := &s
	sp.age = 11
	fmt.Println(s)
	fmt.Println(sp)

	re := rect{}
	re.width = 10
	re.height = 19
	rep := re
	fmt.Println(re.add2())
	fmt.Println(re)
	fmt.Println(rep.add2())
	fmt.Println(rep)
}

type person struct {
	name string
	age  int
}

func zeroval(i int) {
	i = 0
}
func zeropoint(i *int) {
	*i = 0
}

type rect struct {
	width  int
	height int
}

func (a rect) add() int {
	a.height += 1
	a.width -= 1
	return a.height + a.width
}

func (a *rect) add2() int {
	a.height += 1
	a.width -= 1
	return a.height * a.width
}
