package main

import (
	"bufio"
	"fmt"
	"strings"
)

func main() {
	sr := strings.NewReader("ABCDEFGHJKIUYTRFJKEKFHF1234567890")
	buf := bufio.NewReaderSize(sr, 0)

	fmt.Println("==", buf.Buffered())
	s, _ := buf.Peek(5)
	fmt.Printf("%d == %q \n", buf.Buffered(), s)
	nn, err := buf.Discard(3)
	fmt.Println(nn, err)

	fmt.Println()
	b := make([]byte, 10)
	for n, err := 0, error(nil); err == nil; {
		fmt.Printf("Buffered:%d ==Size:%d== n:%d==  b[:n] %q ==  err:%v\n",
			buf.Buffered(), buf.Size(), n, b[:n], err)
		n, err = buf.Read(b)
		fmt.Printf("Buffered:%d ==Size:%d== n:%d==  b[:n] %q ==  err: %v == s: %s\n",
			buf.Buffered(), buf.Size(), n, b[:n], err, s)
		fmt.Println()
	}
	fmt.Printf("%d ==  %q\n", buf.Buffered(), s)
}
