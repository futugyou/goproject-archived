package main

import (
	"fmt"
	"time"
)

func main() {
	bs := make([]byte, 1<<26)
	s0 := string(bs)
	s1 := string(bs)
	s2 := s1

	startTime := time.Now()
	_ = s0 == s1
	duration := time.Now().Sub(startTime)
	fmt.Println("s0==s1 :", duration)

	startTime = time.Now()
	_ = s2 == s1
	duration = time.Now().Sub(startTime)
	fmt.Println("s2==s1 :", duration)
}
