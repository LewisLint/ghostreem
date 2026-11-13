package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	port := flag.String("p", "6969", "port to listen on")
	dir := flag.String("d", ".", "media directory")
	flag.Parse()

	handler, err := newServer(*dir)
	if err != nil {
		log.Fatal(err)
	}

	addr := ":" + *port
	fmt.Printf("Ghostream running at http://localhost%s\n", addr)

	log.Fatal(http.ListenAndServe(addr, handler))
}
