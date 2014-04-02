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
)

var (
	// binary, cross, test, test-integration
	testMethod = os.Getenv("TEST_METHOD")
	store      *dockerci.Store
)

type handler struct {
}

func (h *handler) HandleMessage(msg *nsq.Message) error {
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

	// run make test-integration
	result, err := dockerci.MakeTest(temp, testMethod)
	if err != nil {
		return err
	}

	if err := pushResults(json, result); err != nil {
		return err
	}
	return nil
}

func pushResults(json *simplejson.Json, result *dockerci.Result) error {
	log.Printf("size=%d success=%v\n", len(result.Output), result.Success)

	repoName, sha, err := dockerci.GetRepoNameAndSha(json)
	if err != nil {
		return err
	}
	if err := store.SaveBuildResult(repoName, sha, result.ToData()); err != nil {
		return err
	}
	return nil
}

func main() {
	if testMethod == "" {
		log.Fatalln("TEST_METHOD cannot be empty provide (binary, cross, test, test-integration)")
	}
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("method=%s\n", testMethod)
	reader, err := nsq.NewReader("builds", testMethod)
	if err != nil {
		log.Fatal(err)
	}
	store = dockerci.New(os.Getenv("REDIS"), os.Getenv("REDIS_AUTH"))
	defer store.Close()

	reader.AddHandler(&handler{})
	reader.VerboseLogging = false

	if err := reader.ConnectToNSQ(os.Getenv("NSQD")); err != nil {
		log.Fatal(err)
	}
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
