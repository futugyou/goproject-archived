package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var verbose = flag.Bool("v", false, "show verbose progress message")
var sema = make(chan struct{}, 20)
var done = make(chan struct{})

func canceled() bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

func walkDir(dir string, n *sync.WaitGroup, filesizes chan<- int64) {
	defer n.Done()
	if canceled() {
		return
	}
	for _, entry := range dirents(dir) {
		if entry.IsDir() {
			n.Add(1)
			subdir := filepath.Join(dir, entry.Name())
			go walkDir(subdir, n, filesizes)
		} else {
			filesizes <- entry.Size()
		}
	}
}
func dirents(dir string) []os.FileInfo {
	select {
	case sema <- struct{}{}:
	case <-done:
		return nil // cancelled
	}
	defer func() { <-sema }() // release token
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		//fmt.Fprintf(os.Stderr, "du1: %v\n", err)
		return nil
	}
	return entries
}

//go run src/work/golang-test/golangpro/ch8/du2.go -v  C:\
func main() {
	flag.Parse()
	roots := flag.Args()
	if len(roots) == 0 {
		roots = []string{"."}
	}

	filesizes := make(chan int64)

	var n sync.WaitGroup
	for _, root := range roots {
		n.Add(1)
		go walkDir(root, &n, filesizes)
	}

	go func() {
		n.Wait()
		close(filesizes)
	}()
	go func() {
		os.Stdin.Read(make([]byte, 1))
		close(done)
	}()
	var tick <-chan time.Time
	if *verbose {
		tick = time.Tick(500 * time.Millisecond)
	}
	var nfiles, nbytes int64
loop:
	for {
		select {
		case <-done:
			//在结束之前我们需要把fileSizes channel中的内容“排”空
			for range filesizes {

			}
		case size, ok := <-filesizes:
			if !ok {
				break loop
			}
			nfiles++
			nbytes += size
		case <-tick:
			printDiskUsage(nfiles, nbytes)
		}
	}
	printDiskUsage(nfiles, nbytes)
}
func printDiskUsage(nfiles, nbytes int64) {
	fmt.Printf("%d files  %.1f GB\n", nfiles, float64(nbytes)/1e9)
}
