package main

import (
	"fmt"
	"math"
)

func main() {
	var a = math.NaN()
	fmt.Println(a)

	var m = map[float64]int{}
	m[a] = 123
	v, present := m[a]
	fmt.Println(v, present)

	m[a] = 3123
	v, present = m[a]
	fmt.Println(v, present)

	fmt.Println(m)
	delete(m, a)
	fmt.Println(m)

	for k, v := range m {
		fmt.Println(k, v)
	}

	s := "b"
	x := []byte(s)
	fmt.Println(cap([]byte(s)))
	fmt.Println(cap(x))
	fmt.Println(x)
}
