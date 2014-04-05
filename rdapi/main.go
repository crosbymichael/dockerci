package main

import (
	"github.com/gorilla/mux"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
)

type dockerHandler struct {
	path string
}

func (h *dockerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := h.newConn()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer c.Close()

	res, err := c.Do(r)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	copyHeader(w.Header(), res.Header)
	if _, err := io.Copy(w, res.Body); err != nil {
		log.Println(err)
	}
}

func (h *dockerHandler) newConn() (*httputil.ClientConn, error) {
	conn, err := net.Dial("unix", h.path)
	if err != nil {
		return nil, err
	}
	return httputil.NewClientConn(conn, nil), nil
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func main() {
	r := mux.NewRouter()
	docker := &dockerHandler{os.Getenv("DOCKER_SOCK")}
	for _, route := range []string{
		"/containers/json",
		"/containers/ps",
		"/containers/{name:.*}/json",
		"/containers/{name:.*}/attach",
	} {
		r.Handle("/v{version:[0-9.]+}"+route, docker)
		r.Handle(r, docker)
	}

	if err := http.ListenAndServe(":4243", r); err != nil {
		log.Fatal(err)
	}
}
