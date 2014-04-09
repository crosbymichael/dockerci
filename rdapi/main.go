package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"regexp"
	"strings"
)

type dockerHandler struct {
	path string
}

func (h *dockerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	println(r.RequestURI)
	c, err := h.newConn()
	if err != nil {
		log.Println("err newconn", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer c.Close()

	res, err := c.Do(r)
	if err != nil {
		log.Println("err Do", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	copyHeader(w.Header(), res.Header)
	if _, err := io.Copy(w, res.Body); err != nil {
		log.Println("err cpy:", err)
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

func handleConnection(rwc net.Conn) (statusCode int, err error) {
	defer func() {
		conn := rwc.(*net.TCPConn)
		conn.CloseRead()
		conn.CloseWrite()
		conn.Close()
	}()

	conn, err := net.Dial("unix", "/var/run/docker.sock")
	if err != nil {
		panic(err)
	}

	buf := make([]byte, 2*1024)
	n, err := rwc.Read(buf)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Error reading header from client: %s", err)
	}

	r := bytes.NewBuffer(buf)
	line, _ := bufio.NewReader(r).ReadString('\n')
	tab := strings.Split(line, " ")

	valid := false
	for _, validation := range []*regexp.Regexp{
		regexp.MustCompile(`^(/v[0-9.]+)?/containers/(.*)/json$`),
		regexp.MustCompile(`^(/v[0-9.]+)?/containers/(.*)/attach(\?.*)?$`),
		regexp.MustCompile(`^(/v[0-9.]+)?/containers/json$`),
	} {
		if validation.MatchString(tab[1]) {
			valid = true
			break
		}
	}
	if !valid {
		log.Println("401 Forbidden\r\n")
		return http.StatusNotFound, fmt.Errorf("Unkown or invalid route")
	}
	if _, err := conn.Write(buf[:n]); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Error write headers to remote docker: %s", err)
	}
	c := make(chan struct{}, 2)
	defer close(c)

	go func() {
		io.Copy(conn, rwc)
		conn.(*net.UnixConn).CloseRead()
		c <- struct{}{}
	}()
	go func() {
		io.Copy(rwc, conn)
		rwc.(*net.TCPConn).CloseRead()
		c <- struct{}{}
	}()
	<-c
	<-c

	return http.StatusOK, nil
}

func main() {
	ln, err := net.Listen("tcp", ":4244")
	if err != nil {
		log.Fatalf("Error listen: %s\n", err)
	}
	total := 0
	for n := 1; ; n++ {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Error accept: %s\n", err)
			continue
		}
		go func(n int) {
			total++
			println("------> start of routine", n, "total:", total)
			defer println("<---- end of routine", n, "total:", total)
			defer func() { total -= 1 }()
			if statusCode, err := handleConnection(conn); err != nil {
				if err := (&http.Response{
					StatusCode: statusCode,
					Status:     err.Error(),
				}).Write(conn); err != nil {
					log.Printf("Error sending error to client: %s\n", err)
				}
			}
		}(n)
	}
}

func main2() {
	r := mux.NewRouter()
	docker := &dockerHandler{os.Getenv("DOCKER_SOCK")}
	for _, route := range []string{
		"/containers/json",
		"/containers/ps",
		"/containers/{name:.*}/json",
		"/containers/{name:.*}/attach",
	} {
		r.Handle("/v{version:[0-9.]+}"+route, docker)
		r.Handle(route, docker)
	}

	if err := http.ListenAndServe(":4244", r); err != nil {
		log.Fatal(err)
	}
}
