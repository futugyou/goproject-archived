package main

import (
	"bytes"
	"fmt"
	"regexp"
)

var p = fmt.Println

func main() {
	match, _ := regexp.MatchString("p([a-z]+)ch", "peach")
	p(match)

	r, _ := regexp.Compile("p([a-z]+)ch")

	p(r.MatchString("psssch"))
	p(r.FindAllString("pssch psdoch", 3))
	p(r.FindString("pssch psdoch"))
	p(r.FindStringIndex("pch pach paach"))
	p(r.FindStringSubmatch("pch pach paach"))
	p(r.FindAllStringSubmatch("pch pach paach", -1))
	p(r.Match([]byte("peach")))
	r = regexp.MustCompile("p([a-z]+)ch")
	in := []byte("a peach")
	out := r.ReplaceAllFunc(in, bytes.ToUpper)
	p(string(out))
}
