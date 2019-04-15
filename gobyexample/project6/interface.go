package main

import (
	"fmt"
	"math"
)

func main() {
	a := &rect{1, 2}
	test(a)
}

type geometry interface {
	area() float64
	perim() float64
}

type rect struct {
	width, height float64
}

func (a *rect) area() float64 {
	return math.Pi * a.width * a.height
}

func (a *rect) perim() float64 {
	return a.width * a.height
}

func test(a geometry) {
	fmt.Println(a)
	fmt.Println(a.area())
	fmt.Println(a.perim())
}
