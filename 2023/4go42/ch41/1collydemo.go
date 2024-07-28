package main

import(
	"fmt"
	"github.com/gocolly/colly"
)

func main(){
	c:= colly.NewCollector(
		//colly.AllowedDomains("news.baidu.com")
		colly.UserAgent("Opera/9.80 (Windows NT 6.1; U; zh-cn) Presto/2.9.168 Version/11.50")	)

	c.OnRequest(func(r *colly.Request){
		r.Headers.Set("Host","baidu.com")
		r.Headers.Set("Connection","keep-alive")
		r.Headers.Set("Accept","*/*")
		r.Headers.Set("Origin","")
		r.Headers.Set("Referer","http://www.baidu.com")
		r.Headers.Set("Accept-Encoding","gzip, deflate")
		r.Headers.Set("Accept-Language","zh-CN, zh;q=0.9")
		fmt.Println("visiting",r.URL)
	})

	c.OnHTML("title",func(e *colly.HTMLElement){
		//e.Request.Visit(e.Attr("href"))
		fmt.Println("title:",e.Text)
	})

	c.OnHTML("body",func(e *colly.HTMLElement){
		//<div class="hotnews" alog-group="focistop-hotnews">
		e.ForEach(".hotnew a",func(i int,el *colly.HTMLElement){
			band:=el.Attr("href")
			title:=el.Text
			fmt.Printf("new %d : %s - %s\n",i,title,band)
			//e.Request.Visit(band)
		})
	})

	// c.OnHTML(`.next a[href]`,func(e *colly.HTMLElement){
	// 	e.Request.Visit(e.Attr("href"))
	// })

	c.OnResponse(func(r *colly.Response){
		fmt.Println("response received",r.StatusCode)
		//fmt.Println(r.Ctx.Get("url"))
	})

	c.Limit(&colly.LimitRule{
		Parallelism:2,
		//Delay:5*time.Second,
	})

	c.Visit("http://news.baidu.com")
}