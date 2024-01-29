package main

import (
	"log"
	"net/http"
	"net/http/httputil"
)

func RequestHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("URI %+v Header: %+v TLS: %+v", r.RequestURI, r.Header, r.TLS)
	if req, err := httputil.DumpRequest(r, true); err == nil {
		w.Write(req)
	}
}

func main() {
	http.HandleFunc("/predict", RequestHandler)
	http.HandleFunc("/upload", RequestHandler)
	http.ListenAndServe(":8888", nil)
}
