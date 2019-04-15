package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	dat, err := ioutil.ReadFile("tmp/dat")
	check(err)
	fmt.Println(string(dat))
	f, err := os.Open("tmp/dat")
	check(err)

	b1 := make([]byte, 5)
	n1, err := f.Read(b1)
	check(err)
	fmt.Printf("%d bytes: %s\n", n1, string(b1))

	o2, err := f.Seek(6, 0)
	check(err)
	b2 := make([]byte, 2)
	n2, err := f.Read(b2)
	check(err)
	fmt.Printf("%d bytes @ %d: %s\n", n2, o2, string(b2))

	o3, err := f.Seek(-2, 1)
	check(err)
	b3 := make([]byte, 4)
	n3, err := io.ReadAtLeast(f, b3, 4)
	check(err)
	fmt.Printf("%d bytes @ %d: %s\n", n3, o3, string(b3))

	_, err = f.Seek(0, 0)
	check(err)

	r4 := bufio.NewReader(f)
	b4, err := r4.Peek(5)
	check(err)
	fmt.Printf("5 bytes: %s\n", string(b4))
	f.Close()

	d1 := []byte("hello\ngo\n")
	err = ioutil.WriteFile("tmp/dat1.txt", d1, 0644)
	check(err)

	ff, err := os.Create("tmp/dat2")
	check(err)
	defer ff.Close()

	d2 := []byte{115, 111, 109, 101, 10}
	nn2, err := ff.Write(d2)
	check(err)
	fmt.Printf("wrote %d bytes\n", nn2)

	nn3, err := ff.WriteString("this is test\n")
	fmt.Printf("wrote %d bytes\n", nn3)
	ff.Sync()

	w := bufio.NewWriter(ff)
	n4, err := w.WriteString("buffer\n")
	fmt.Printf("wrote %d bytes\n", n4)
	w.Flush()

	scanner := bufio.NewScanner(os.Stdin)
	//cat tmp/dat2 | go run src/work/project26/file-dome.go
	for scanner.Scan() {
		ucl := strings.ToUpper(scanner.Text())
		fmt.Println(ucl)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

}
