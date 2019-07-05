package main

import (
	example "work/golang-test/6gorpc/ch1/service"

	"context"
	"log"

	"github.com/smallnest/rpcx/client"
)

func main() {
	d := client.NewPeer2PeerDiscovery("tcp@127.0.0.1:7890", "")

	xclient := client.NewXClient("Arith", client.Failtry, client.RandomSelect, d, client.DefaultOption)
	defer xclient.Close()

	args := &example.Args{
		A: 10,
		B: 20,
	}
	reply := &example.Reply{}

	err := xclient.Call(context.Background(), "Mul", args, reply)
	//call,	err := xclient.go(context.Background(), "Mul", args, reply)
	// <-call.Done
	if err != nil {
		log.Fatalf("failed to cal: %v", err)
	}

	log.Printf("%d * %d = %d", args.A, args.B, reply.C)
}
