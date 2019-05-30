package main

import (
	"fmt"
)

func main() {
	s1 := []int{1, 2, 3}
	fmt.Println(len(s1), cap(s1), s1)
	s2 := s1[1:]
	fmt.Println(len(s2), cap(s2), s2)
	for i := range s2 {
		s2[i] += 20
	}
	fmt.Println(len(s1), cap(s1), s1)
	fmt.Println(len(s2), cap(s2), s2)

	s2 = append(s2, 4)
	for i := range s2 {
		s2[i] += 20
	}
	fmt.Println(len(s1), cap(s1), s1)
	fmt.Println(len(s2), cap(s2), s2)

	data := []int{1, 2, 3}
	for _, v := range data {
		v = v * 10
	}
	fmt.Println(data)

	for v, _ := range data {
		data[v] = data[v] * 2
	}
	fmt.Println(data)
}
