package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"syscall"
)

type (
	closeReader interface {
		CloseRead() error
	}
	closeWriter interface {
		CloseWrite() error
	}
)

func goTransfert(dst io.Writer, src io.Reader) chan error {
	c := make(chan error)
	go func() {
		_, err := io.Copy(dst, src)
		e1 := dst.(closeReader).CloseRead()
		if err != nil {
			c <- err
		} else {
			c <- e1
		}
	}()
	return c
}

type dockerProxy struct {
	allowedPath []*regexp.Regexp
}

func (dp *dockerProxy) handleConnection(srcConn io.ReadWriteCloser, toProto, toAddr string) (statusCode int, err error) {
	toConn, err := net.Dial(toProto, toAddr)
	if err != nil {
		return http.StatusGone, fmt.Errorf("requested docker not available")
	}

	buf := make([]byte, 2*1024)
	n, err := srcConn.Read(buf)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("read header from client: %s", err)
	}

	r := bytes.NewBuffer(buf)
	line, _ := bufio.NewReader(r).ReadString('\n') // if this fail, split will fail and return error
	tab := strings.Split(line, " ")
	if len(tab) != 3 {
		return http.StatusInternalServerError, fmt.Errorf("invalid request")
	}
	url := tab[1]
	valid := false
	for _, validation := range dp.allowedPath {
		if validation.MatchString(url) {
			valid = true
			break
		}
	}
	if !valid {
		return http.StatusNotFound, fmt.Errorf("Unkown or invalid route: %s", url)
	}
	if _, err := toConn.Write(buf[:n]); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("write headers to remote docker: %s", err)
	}

	c1 := goTransfert(toConn, srcConn)
	defer close(c1)
	c2 := goTransfert(srcConn, toConn)
	defer close(c2)

	select {
	case <-c1:
	case <-c2:
	}
	select {
	case <-c1:
	case <-c2:
	}

	return http.StatusOK, nil
}

/*
   ListenAndServe starts the proxy from the given proto/addr to the given proto/addr.
*/
func (dp *dockerProxy) ListenAndServe(fromProto, fromAddr string, toProto, toAddr string) error {
	ln, err := net.Listen(fromProto, fromAddr)
	if err != nil {
		return fmt.Errorf("listen: %s", err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Error accept: %s\n", err)
			continue
		}
		go func() {
			defer conn.Close()
			if statusCode, err := dp.handleConnection(conn, toProto, toAddr); err != nil {
				fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\nConnection: Close\r\nContent-Length: 0\r\n\r\n", statusCode, "error")
				log.Printf("Error handleConnection: %s (%d)\n", err, statusCode)
			}
		}()
	}
	return nil
}

/*
   AllowPath adds a path represented by the regpext `expr` in the whitelist.
   if `strict` is true, then the exact expr is used, otherwise it allows
   for query parameters.
   Regardless of `strict`, the expr will be prefixed with the optional /vN.NN
*/
func (dp *dockerProxy) AllowPath(expr string, strict bool) error {
	if dp.allowedPath == nil {
		dp.allowedPath = make([]*regexp.Regexp, 0)
	}
	if !strict {
		expr += `(\?.*)?`
	}
	c, err := regexp.Compile(`^(/v[0-9.]+)?` + expr + `$`)
	if err != nil {
		return err
	}
	dp.allowedPath = append(dp.allowedPath, c)
	return nil
}

func displayDebug() error {
	// Dislpay the amount of open fds
	fds, err := ioutil.ReadDir(fmt.Sprintf("/proc/%d/fd", os.Getpid()))
	if err != nil {
		log.Printf("Error reading fd tree: %s\n", err)
		return err
	}
	fmt.Printf("Open fds: %d\n", len(fds))

	// Display the number of gorountines
	fmt.Printf("Running goroutines: %d\n", runtime.NumGoroutine())

	// Display memory stats
	runtime.GC() // Make sure the GC runs before lookup up the stats
	ms := &runtime.MemStats{}
	runtime.ReadMemStats(ms)
	fmt.Printf("Memory in use: %d, Memory allocated: %d\n", ms.Alloc)
	return nil
}

func startDebug() {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGUSR1)
	for _ = range c {
		go displayDebug()
	}
}

func main() {
	go startDebug()
	dp := &dockerProxy{}
	dp.AllowPath("/containers/(.*)/json", true)
	dp.AllowPath("/containers/json", true)
	dp.AllowPath("/containers/(.*)/attach", false)
	if err := dp.ListenAndServe("tcp", ":4244", "unix", "/var/run/docker.sock"); err != nil {
		log.Fatalf("listenAndServe error: %s\n", err)
	}
}
