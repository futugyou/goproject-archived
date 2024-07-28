package main

import (
	"fmt"
	"strings"
)

func main() {
	var s = "abaay森z众xbbab"
	o := fmt.Println
	o(strings.TrimPrefix(s, "ab"))
	o(strings.TrimSuffix(s, "ab"))
	o(strings.TrimLeft(s, "ba"))  //y森z众xbbab
	o(strings.TrimRight(s, "ba")) //abaay森z众x
	o(strings.Trim(s, "b a"))     //y森z众x
	o(strings.TrimFunc(s,func(r rune)bool{
		o(r)
		return r<128
	}))
}
