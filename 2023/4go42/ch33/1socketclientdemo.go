package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"time"
)

func main() {
	var tcpAddr *net.TCPAddr
	tcpAddr, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:999")

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		fmt.Println("client connect error! :" + err.Error())
		return
	}

	defer conn.Close()

	fmt.Println(conn.LocalAddr().String() + " : client connected!")

	onMessageRecived(conn)
}

func onMessageRecived(conn *net.TCPConn) {
	reader := bufio.NewReader(conn)
	b := []byte(conn.LocalAddr().String() + " say hello to server ...\n")
	conn.Write(b)
	for { 
		msg, err := reader.ReadString('\n')
		fmt.Println("ReadString")
		fmt.Println(msg)

		if err != nil || err == io.EOF {
			fmt.Println(err)
			break
		}

		time.Sleep(time.Second * 2)
		fmt.Println("writing ...")
		b := []byte(conn.LocalAddr().String() + " write data to server...\n")
		_, err = conn.Write(b)
		if err != nil {
			fmt.Println(err)
			break
		}
	}
}
