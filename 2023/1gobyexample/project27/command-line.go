package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	args := os.Args
	argswithoutprog := os.Args[1:]
	fmt.Println(args)
	fmt.Println(argswithoutprog)

	wordptr := flag.String("word", "bbb", "a string")
	numptr := flag.Int("numb", 42, "an int")
	boolptr := flag.Bool("fork", false, "a bool")
	var svar string
	flag.StringVar(&svar, "svar", "bar", "a string var")
	flag.Parse()
	fmt.Println("word:", *wordptr)
	fmt.Println("numb:", *numptr)
	fmt.Println("fork:", *boolptr)
	fmt.Println("svar:", svar)
	fmt.Println("tail:", flag.Args())
	/// go run src/work/project27/command-line.go -word sd -numb 3 -fork true -svar s sss

	os.Setenv("FOO", "1")
	fmt.Println("FOO:", os.Getenv("FOO"))
	fmt.Println("BAR:", os.Getenv("BAR"))
	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		fmt.Println(pair)
	}
}
