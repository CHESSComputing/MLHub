package main

import (
	"io"
	"log"
	"net/http"
	"net/http/httputil"
)

func RequestHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("URI %+v Header: %+v TLS: %+v", r.RequestURI, r.Header, r.TLS)
	if r.Header.Get("Accept") == "application/json" {
		data, err := io.ReadAll(r.Body)
		if err == nil {
			w.Write(data)
		} else {
			w.Write([]byte(err.Error()))
		}
		return
	}
	if req, err := httputil.DumpRequest(r, true); err == nil {
		w.Write(req)
	}
}

func main() {
	http.HandleFunc("/predict", RequestHandler)
	http.HandleFunc("/upload", RequestHandler)
	port := ":8888"
	log.Println("Start GoFake HTTP server on port", port)
	http.ListenAndServe(port, nil)
}
