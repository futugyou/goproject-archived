package main

import (
	"fmt"
	"regexp"
	"time"

	"github.com/gocolly/colly"
)

var visited = map[string]bool{}

func main() {
	c := colly.NewCollector(
		colly.AllowedDomains("https://books.studygolang.com/"),
		colly.MaxDepth(1),
	)

	detailRegex, _ := regexp.Compile(`/go/go\?p=\d+$`)
	listRegex, _ := regexp.Compile(`/t/\d+#\w+`)

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")

		if visited[link] && (detailRegex.Match([]byte(link)) || listRegex.Match([]byte(link))) {
			return
		}

		if !detailRegex.Match([]byte(link)) && !listRegex.Match([]byte(link)) {
			fmt.Println("not match :", link)
			return
		}

		time.Sleep(time.Second)
		fmt.Println("match :", link)

		visited[link] = true

		time.Sleep(time.Millisecond * 10)
		c.Visit(e.Request.AbsoluteURL(link))
	})

	err := c.Visit("https://books.studygolang.com/")
	if err != nil {
		fmt.Println(err)
	}
}
