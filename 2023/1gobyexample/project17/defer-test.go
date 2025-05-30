package main

import (
	"fmt"
	"os"
	"path"
)

func main() {
	f := createFile("./tmp/defer.txt")
	defer closeFile(f)
	writeFile(f)
}
func createFile(p string) *os.File {
	fmt.Println("creating")
	pa := path.Dir(p)
	fmt.Println(pa)
	os.MkdirAll(pa, os.ModePerm)
	f, err := os.Create(p)
	if err != nil {
		panic(err)
	}
	return f
}

func writeFile(f *os.File) {
	fmt.Println("writing")
	fmt.Fprintln(f, "data")
}
func closeFile(f *os.File) {
	fmt.Println("closinng")
	f.Close()
}
