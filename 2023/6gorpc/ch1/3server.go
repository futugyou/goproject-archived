package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"time"
	arg "work/golang-test/6gorpc/ch1/service"

	"github.com/smallnest/rpcx/server"
)

var clientConn net.Conn
var connected = false

type Arith int

func (t *Arith) Mul(ctx context.Context, args *arg.Args, reply *arg.Reply) error {
	clientConn = ctx.Value(server.RemoteConnContextKey).(net.Conn)
	reply.C = args.A * args.B
	connected = true
	return nil
}

func main() {
	in, _ := net.Listen("tcp", ":9981")
	go http.Serve(in, nil)

	s := server.NewServer()
	//s.RegisterName("Arith",new(arg.Arith),"")
	s.Register(new(Arith), "")

	go s.Serve("tcp", "127.0.0.1:8972")

	for !connected {
		time.Sleep(time.Second)
	}

	fmt.Printf("start to send messages to %s\n", clientConn.RemoteAddr().String())

	for {
		if clientConn != nil {
			err := s.SendMessage(clientConn, "test_service_path", "test_service_method", nil, []byte("abcde"))
			if err != nil {
				fmt.Printf("failed to send messsage to %s: %v\n", clientConn.RemoteAddr().String(), err)
				clientConn = nil
			}
		}
		time.Sleep(time.Second)
	}
}
