package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

func main() {
	service := "127.0.0.1:8888"
	tcpAddr, err := net.ResolveTCPAddr("tcp4", service)
	checkError(err)
	listener, err := net.ListenTCP("tcp", tcpAddr)
	checkError(err)
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go WriteResponse(conn)

	}
}

func WriteResponse(conn net.Conn) {
	daytime := time.Now().String()
	fmt.Println(daytime)
	conn.Write([]byte(daytime))
	defer conn.Close()
}
