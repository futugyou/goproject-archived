package main

import (
	"fmt"
	"net"
	"os"
	"strings"
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
	conn.SetReadDeadline(time.Now().Add(2 * time.Minute))
	request := make([]byte, 128)
	defer conn.Close()

	for {
		read_len, err := conn.Read(request)
		if err != nil {
			fmt.Println("this is err : ",err)
			break
		}
		if read_len == 0 {
			break
		} else if strings.TrimSpace(string(request[:read_len])) == "timestamp" {
			daytime := time.Now().String()
			fmt.Println(daytime)
			conn.Write([]byte(daytime))
		} else {
			daytime := time.Now().Add(time.Hour).String()
			fmt.Println(daytime)
			conn.Write([]byte(daytime))
		}
		request = make([]byte, 128) // clear last read content
	}

}
