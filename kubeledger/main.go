package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

var (
	portvar int
)

func init() {
	flag.IntVar(&portvar, "p", 9096, "the base port for the server")
}

func main() {
	flag.Parse()

	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("js"))))
	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("css"))))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/dataset/", http.StripPrefix("/dataset/", http.FileServer(http.Dir("static/data"))))

	http.HandleFunc("/", renderHandler)
	http.HandleFunc("/api/nodes/heatmap", getNodeHeatmapHandler)

	log.Printf("Server is running at %d port.\n", portvar)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", portvar), nil))
}
