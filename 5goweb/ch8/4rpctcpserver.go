package main

import (
	"errors"
	"fmt"
	"net"
	"net/rpc"
)

type Args struct {
	A, B int
}

type Quotient struct {
	Quo, Rem int
}

type Arith int

func (t *Arith) Multiply(args *Args, reply *int) error {
	*reply = args.A * args.B
	return nil
}
func (t *Arith) Divide(args *Args, quo *Quotient) error {
	if args.B == 0 {
		return errors.New("divide by zero")
	}
	quo.Quo = args.A / args.B
	quo.Rem = args.A % args.B
	return nil
}

func main() {
	arith := new(Arith)
	rpc.Register(arith)

	//rpc.HandleHTTP()

	tcpAddr, err := net.ResolveTCPAddr("tcp", ":1234")
	listener, err := net.ListenTCP("tcp", tcpAddr)
	//err = http.ListenAndServe(":1234", nil)
	if err != nil {
		fmt.Println(err.Error())
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		rpc.ServeConn(conn)
	}
}
