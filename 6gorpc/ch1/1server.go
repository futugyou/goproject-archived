package main

import (
	arg "work/golang-test/6gorpc/ch1/service"

	"github.com/smallnest/rpcx/server"
)

func main() {
	s := server.NewServer()
	s.RegisterName("Arith", new(arg.Arith), "")
	//s.Register(new(arg.Arith), "")
	s.Serve("tcp", ":7890")
}
