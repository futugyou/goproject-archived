package main

import "fmt"

type coder interface {
	code()
	debug()
}
type Gopher struct {
	language string
}

func (p Gopher) code() {
	fmt.Printf("coding %s \n", p.language)
}
func (p *Gopher) debug() {
	fmt.Printf("debuging %s \n", p.language)
	p.language = "php"
}
func main() {
	var c coder = &Gopher{"GO"}
	c.debug()
	c.code()
}
