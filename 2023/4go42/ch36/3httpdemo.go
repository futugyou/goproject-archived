package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

type timeHandler struct {
	format string
}

func (th *timeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tm := time.Now().Format(th.format)
	w.Header().Set("Content-Type", "text/html")
	html := `<doctype html>
        <html>
        <head>
		  <title>Hello World</title>
        </head>
        <body>
        <p>
          Welcome
		</p>		
		<h1>` + tm + `</h1>
        </body>
		</html>`
	fmt.Fprintln(w, html)
	w.Write([]byte("the time is :" + tm))
}

func main() {
	mux := http.NewServeMux()
	th := &timeHandler{format: time.RFC1123}
	mux.Handle("/time", th)

	log.Println("listening...")
	http.ListenAndServe(":3000", mux)
}
