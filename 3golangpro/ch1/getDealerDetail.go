package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

const baseUrl = "https://bmwdealerint.chinacloudsites.cn/api/"

func main() {
	ch := make(chan string)
	urls := []string{
		"Maintenance?brand=bmw&model=f12_730li&miles=10000",
		"LatestDealer?usid=8159c8c9-eafc-4f55-9ba1-d1c20d1323ac",
		"OrderDetail?id=1436"}
	start := time.Now()
	for _, url := range urls {
		go getDealerDetail(url, ch)
	}

	for range urls {
		fmt.Println()
		fmt.Println(<-ch)
	}
	fmt.Printf("\ntotal time : %.2fs ", time.Since(start).Seconds())
}

func getDealerDetail(url string, ch chan<- string) {
	start := time.Now()
	client := &http.Client{}
	reqest, err := http.NewRequest("GET", baseUrl+url, nil)

	reqest.Header.Add("x-btcapi-usid", "8159c8c9-eafc-4f55-9ba1-d1c20d1323ac")
	reqest.Header.Add("X-AppKey", "2014_MyBMW837")
	if err != nil {
		ch <- fmt.Sprint(err)
		return
	}
	resp, err := client.Do(reqest)
	if err != nil {
		ch <- fmt.Sprint(err)
		return
	}
	defer resp.Body.Close()
	//nbytes, err := io.Copy(ioutil.Discard, resp.Body)

	b, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		ch <- fmt.Sprint(err)
		return
	}
	secs := time.Since(start).Seconds()
	ch <- fmt.Sprintf("%.2fs\t%s", secs, b)
}
