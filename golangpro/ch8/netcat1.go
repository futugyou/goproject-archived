package main

import (
	"io"
	"log"
	"net"
	"os"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:8000")
	n := conn.(*net.TCPConn)
	if err != nil {
		log.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		if _, err := io.Copy(os.Stdout, conn); err != nil {
			log.Fatal(err)
		}
		n.CloseRead()
		log.Println("done")
		done <- struct{}{}
	}()
	mustCopy(conn, os.Stdin)
	n.CloseWrite()
	<-done
}
func mustCopy(dst io.Writer, src io.Reader) {
	if _, err := io.Copy(dst, src); err != nil {
		log.Fatal(err)
	}
}
