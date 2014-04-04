package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type dockerHandler struct {
}

func newTcpHandler(addr string) http.Handler {
	u, err := url.Parse(addr)
	if err != nil {
		log.Fatal(err)
	}
	return httputil.NewSingleHostReverseProxy(u)
}
