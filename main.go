package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

var addr = flag.String("addr", ":80", "http service address")

func main() {
	flag.Parse()
	hub := newHub()
	go hub.run()

	publishMessageHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		publishWs(hub, w, r)
	})

	r := mux.NewRouter()
	r.Handle(`/publish/{key:.*}`, tokenAuthMiddleware(publishMessageHandler))
	r.HandleFunc(`/{key:.*}`, func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	http.Handle("/", r)
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
