package main

import (
	"fmt"
	"log"
	"os"
	"work/golang-test/golangpro/ch5/links"
)

func crawl(url string) []string {
	fmt.Println(url)
	list, err := links.Extract(url)
	if err != nil {
		log.Print(err)
	}
	return list
}

func main() {
	worklist := make(chan []string)
	unseenlinks := make(chan string)

	go func() {
		worklist <- os.Args[1:]
	}()

	for i := 0; i < 20; i++ {
		go func() {
			for link := range unseenlinks {
				foundlinks := crawl(link)
				go func() {
					//不开goroutine会死锁
					worklist <- foundlinks
				}()
			}
		}()
	}

	seen := make(map[string]bool)
	for list := range worklist {
		for _, link := range list {
			if !seen[link] {
				seen[link] = true
				unseenlinks <- link
			}
		}
	}
}
