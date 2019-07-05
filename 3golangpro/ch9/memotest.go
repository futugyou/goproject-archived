package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
	memo "work/golang-test/golangpro/ch9/memo1"
)

func httpGetBody(url string) (interface{}, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
func main() {
	m := memo.New(httpGetBody)
	var n sync.WaitGroup
	for _, url := range incomingURLs() {
		n.Add(1)
		go func(url string) {
			start := time.Now()
			value, err := m.Get(url)
			if err != nil {
				log.Print(err)
			}
			fmt.Printf("%s, %s, %d bytes\n",
				url, time.Since(start), len(value.([]byte)))
			n.Done()
		}(url)
	}
	n.Wait()
}
func incomingURLs() []string {
	return []string{"https://docs.hacknode.org/gopl-zh/ch9/ch9-07.html",
		"https://docs.hacknode.org/gopl-zh/ch9/ch9-06.html",
		"https://docs.hacknode.org/gopl-zh/ch9/ch9-05.html"}
}
