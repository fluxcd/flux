package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

var rev = "devel"

func main() {
	var (
		addr = flag.String("addr", ":80", "HTTP listen address")
		msg  = flag.String("msg", "Hello, world", "Message to print")
	)
	flag.Parse()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		fmt.Fprintf(w, *msg+"\n")
	})
	log.Printf("%s: listening on %s", rev, *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
