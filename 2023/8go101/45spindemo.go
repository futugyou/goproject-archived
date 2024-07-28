package main

import "fmt"

func main() {
	type S []S
	type M map[string]M
	type C chan C
	type F func(F) F

	s := S{0: nil}
	s[0] = s
	m := M{"GO": nil}
	m["GO"] = m
	c := make(C, 3)
	c <- c
	c <- c
	c <- c
	var f F
	f = func(F) F {
		fmt.Println(1)
		return f
	}

	//runtime: goroutine stack exceeds 1000000000-byte limit fatal error: stack overflow
	//x := s[0][0][0]
	//fmt.Println(x)
	//y := m["GO"]["GO"]["GO"]["GO"]["GO"]
	//fmt.Println(y)
	_ = s[0][0][0]
	_ = m["GO"]["GO"]["GO"]["GO"]["GO"]
	<-<-<-c
	f(f(f(f)))
}
