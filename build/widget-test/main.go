package main

import (
	"log"
	"net/http"
)

func main() {
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)
	log.Println("Widget test server running at http://localhost:8888")
	log.Println("Open in browser to test widget embedding")
	log.Fatal(http.ListenAndServe(":8888", nil))
}
