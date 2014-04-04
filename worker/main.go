package main

import (
	"fmt"
	"github.com/bitly/go-nsq"
	"github.com/bitly/go-simplejson"
	"github.com/crosbymichael/dockerci"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	// binary, cross, test, test-integration
	testMethod = os.Getenv("TEST_METHOD")
	store      *dockerci.Store
)

type handler struct {
}

func (h *handler) HandleMessage(msg *nsq.Message) error {
	defer dockerci.LogTime(store, "build", time.Now())

	json, err := simplejson.NewJson(msg.Body)
	if err != nil {
		return err
	}
	pullrequest := json.Get("pull_request")

	number, err := pullrequest.Get("number").Int()
	if err != nil {
		return err
	}

	// checkout the code in a temp dir
	temp, err := ioutil.TempDir("", fmt.Sprintf("pr-%d", number))
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)

	if err := dockerci.Checkout(temp, pullrequest); err != nil {
		return err
	}
	result, err := dockerci.MakeTest(temp, testMethod)
	if err != nil {
		return err
	}
	return pushResults(json, result)
}

func pushResults(json *simplejson.Json, result *dockerci.Result) error {
	log.Printf("size=%d success=%v\n", len(result.Output), result.Success)
	if !result.Success {
		log.Println(result.Output)
	}

	sha, err := dockerci.GetSha(json)
	if err != nil {
		return err
	}
	if err := store.SaveBuildResult(sha, result.ToData()); err != nil {
		return err
	}
	return nil
}

func validateConfiguration() {
	if testMethod == "" {
		log.Fatalln("TEST_METHOD cannot be empty provide (binary, cross, test, test-integration)")
	}
}

func newReader() *nsq.Reader {
	var (
		nsqd       = os.Getenv("NSQD")
		nsqlookupd = os.Getenv("NSQ_LOOKUPD")
	)
	reader, err := nsq.NewReader("builds", testMethod)
	if err != nil {
		log.Fatal(err)
	}
	reader.AddHandler(&handler{})
	reader.VerboseLogging = false

	switch {
	case nsqd != "":
		if err := reader.ConnectToNSQ(nsqd); err != nil {
			log.Fatal(err)
		}
	case nsqlookupd != "":
		if err := reader.ConnectToLookupd(nsqlookupd); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("you must specify NSQD or NSQ_LOOKUP env vars")
	}
	return reader
}

func main() {
	validateConfiguration()
	log.Printf("method=%s\n", testMethod)

	var (
		sigChan = make(chan os.Signal, 1)
		reader  = newReader()
	)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	store = dockerci.New(os.Getenv("REDIS"), os.Getenv("REDIS_AUTH"))
	defer store.Close()

	for {
		select {
		case <-reader.ExitChan:
			return
		case <-sigChan:
			// if we receive a sig then stop the reader
			reader.Stop()
		}
	}
}
