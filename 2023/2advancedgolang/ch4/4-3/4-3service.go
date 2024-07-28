package main

import (
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	watch "work/golang-test/advancedgolang/ch4/4-3/watchtest"
)

func main() {
	rpc.RegisterName("KVStoreService", watch.NewKVStoreService())

	listener, err := net.Listen("tcp", ":1234")
	if err != nil {
		log.Fatal("ListenTCP error:", err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal("Accept error:", err)
		}
		go rpc.ServeCodec(jsonrpc.NewServerCodec(conn))
	}
}
