package main

import (
	"fmt"
	"github.com/bitly/go-nsq"
	"github.com/bitly/go-simplejson"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
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

	if err := checkout(temp, pullrequest); err != nil {
		return err
	}

	// run make test-integration
	output, err := makeTest(temp)
	if err != nil {
		return err
	}

	if err := pushResults(pullrequest, output); err != nil {
		return err
	}
	return nil
}

func checkout(temp string, pr *simplejson.Json) error {
	// git clone -qb master https://github.com/upstream/docker.git our-temp-directory
	base := pr.Get("base")
	ref, err := base.Get("ref").String()
	if err != nil {
		return err
	}
	url, err := base.Get("repo").Get("url").String()
	if err != nil {
		return err
	}
	log.Printf("ref=%s url=%s\n", ref, url)

	cmd := exec.Command("git", "clone", "-qb", ref, url, temp)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(string(output))
		return err
	}

	head := pr.Get("head")
	url, err = head.Get("repo").Get("url").String()
	if err != nil {
		return err
	}
	ref, err = head.Get("ref").String()
	if err != nil {
		return err
	}
	log.Printf("ref=%s url=%s\n", ref, url)
	// cd our-temp-directory && git pull -q https://github.com/some-user/docker.git some-feature-branch
	cmd = exec.Command("git", "pull", "-q", url, ref)
	cmd.Dir = temp
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Println(string(output))
		return err
	}
	return nil
}

func makeTest(temp string) ([]byte, error) {
	cmd := exec.Command("make", "binary") // just testing binary for now
	cmd.Dir = temp

	output, err := cmd.CombinedOutput()
	if err != nil {
		// it's ok for the make command to return a non-zero exit
		// incase of a failed build
		if _, ok := err.(*exec.ExitError); !ok {
			return output, err
		}
	}
	return output, nil
}

func pushResults(pr *simplejson.Json, output []byte) error {
	return nil
}

func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	reader, err := nsq.NewReader("builds", "binary")
	if err != nil {
		log.Fatal(err)
	}
	reader.AddHandler(&handler{})

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
